package server

import (
	"bytes"
	"container/list"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/ngaut/gearmand/common"
	log "github.com/ngaut/logging"
	"io"
	"net"
	"os"
	"strconv"
	"time"
)

var (
	invalidMagic = errors.New("invalid magic")
	invalidArg   = errors.New("invalid argument")
)

const (
	ctrlCloseSession = 1000
)

var (
	startJid        int64 = 0
	jobHandlePrefix string
	respMagic       = []byte(common.ResStr)
	jidCh           = make(chan string, 50)
)

func init() {
	hn, err := os.Hostname()
	if err != nil {
		hn = os.Getenv("HOSTNAME")
	}

	if hn == "" {
		hn = "localhost"
	}

	workerNameStr := fmt.Sprintf("%s-%d", hn, os.Getpid())
	//cache fmtstr
	jobHandlePrefix = fmt.Sprintf("H:%s:", workerNameStr)
	go func() {
		for {
			jidCh <- genJid()
		}
	}()
}

func genJid() string {
	startJid++
	return jobHandlePrefix + strconv.FormatInt(startJid, 10)
}

func allocJobId() string {
	return <-jidCh
}

type event struct {
	tp            uint32
	args          *Tuple
	result        chan interface{}
	fromSessionId int64
	jobHandle     string
}

type jobworkermap struct {
	workers *list.List
	jobs    *list.List
}

type Tuple struct {
	t0, t1, t2, t3, t4, t5 interface{}
}

func decodeArgs(cmd uint32, buf []byte) ([][]byte, bool) {
	argc := common.ArgCount(cmd)
	//log.Debug("cmd:", common.CmdDescription(cmd), "details:", buf)
	if argc == 0 {
		return nil, true
	}

	args := make([][]byte, 0, argc)

	if argc == 1 {
		args = append(args, buf)
		return args, true
	}

	endPos := 0
	cnt := 0
	for ; cnt < argc-1 && endPos < len(buf); cnt++ {
		startPos := endPos
		pos := bytes.IndexByte(buf[startPos:], 0x0)
		if pos == -1 {
			log.Warning("invalid protocol")
			return nil, false
		}
		endPos = startPos + pos
		args = append(args, buf[startPos:endPos])
		endPos++
	}

	args = append(args, buf[endPos:]) //option data
	cnt++

	if cnt != argc {
		log.Errorf("argc not match %d-%d", argc, len(args))
		return nil, false
	}

	return args, true
}

func sendReply(out chan []byte, tp uint32, data [][]byte) {
	out <- constructReply(tp, data)
}

func constructReply(tp uint32, data [][]byte) []byte {
	buf := &bytes.Buffer{}
	buf.Write(respMagic)

	err := binary.Write(buf, binary.BigEndian, tp)
	if err != nil {
		panic("should never happend")
	}

	length := 0
	for i, arg := range data {
		length += len(arg)
		if i < len(data)-1 {
			length += 1
		}
	}

	err = binary.Write(buf, binary.BigEndian, uint32(length))
	if err != nil {
		panic("should never happend")
	}

	for i, arg := range data {
		buf.Write(arg)
		if i < len(data)-1 {
			buf.WriteByte(0x00)
		}
	}

	return buf.Bytes()
}

func validCmd(cmd uint32) bool {
	if cmd >= common.CAN_DO && cmd <= common.SUBMIT_JOB_EPOCH {
		return true
	}

	if cmd != 39 { //filter gearmand
		log.Warningf("invalid cmd %d", cmd)
	}

	return false
}

func bytes2str(o interface{}) string {
	return string(o.([]byte))
}

func bool2bytes(b bool) []byte {
	if b {
		return []byte{'1'}
	}

	return []byte{'0'}
}

func int2bytes(n int) []byte {
	return []byte(strconv.Itoa(n))
}

func ReadMessage(r io.Reader) (uint32, []byte, error) {
	_, tp, size, err := readHeader(r)
	if err != nil {
		return 0, nil, err
	}

	buf := make([]byte, size)
	_, err = io.ReadFull(r, buf)

	return tp, buf, err
}

func readHeader(r io.Reader) (magic uint32, tp uint32, size uint32, err error) {
	magic, err = readUint32(r)
	if err != nil {
		return
	}

	if magic != common.Req && magic != common.Res {
		log.Debugf("magic not match %v", magic)
		err = invalidMagic
		return
	}

	tp, err = readUint32(r)
	if err != nil {
		return
	}

	if !validCmd(tp) {
		//gearman's bug, as protocol, we should treat this an error, but gearman allow it
		if tp == 39 { //wtf: benchmark worker send this, and i can not find it in protocol description
			tp = common.GRAB_JOB_UNIQ
			size, err = readUint32(r)
			return
		}
		err = invalidArg
		return
	}

	size, err = readUint32(r)

	return
}

func clearOutbox(outbox chan []byte) {
	for {
		select {
		case b, ok := <-outbox:
			if !ok { //channel is empty
				return
			}

			_ = b //compiler bug (tip)
		}
	}
}

func writer(conn net.Conn, outbox chan []byte) {
	defer func() {
		conn.Close()
		clearOutbox(outbox) //incase reader is blocked
	}()

	b := bytes.NewBuffer(nil)

	for {
		select {
		case msg, ok := <-outbox:
			if !ok {
				return
			}
			b.Write(msg)
			for n := len(outbox); n > 0; n-- {
				b.Write(<-outbox)
			}

			conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			_, err := conn.Write(b.Bytes())
			if err != nil {
				return
			}
			b.Reset()
		}
	}
}

func readUint32(r io.Reader) (uint32, error) {
	var value uint32
	err := binary.Read(r, binary.BigEndian, &value)
	return value, err
}

func ValidProtocolDef() {
	if common.CAN_DO != 1 || common.SUBMIT_JOB_EPOCH != 36 { //protocol check
		panic("protocol define not match")
	}
}

func createResCh() chan interface{} {
	return make(chan interface{}, 1)
}

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
	"sync/atomic"
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
	startJid      int64 = 0
	workerNameStr string
)

func init() {
	hn, err := os.Hostname()
	if err != nil {
		hn = os.Getenv("HOSTNAME")
	}

	if hn == "" {
		hn = "localhost"
	}

	workerNameStr = fmt.Sprintf("%s-%d", hn, os.Getpid())
}

func allocJobId() string {
	jid := atomic.AddInt64(&startJid, 1)
	return fmt.Sprintf("H:%s:%d", workerNameStr, jid)
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
	first, second, third, fourth, fifth, sixth interface{}
}

func decodeArgs(cmd uint32, buf []byte) ([][]byte, bool) {
	argc := common.ArgCount(cmd)
	endPos := 0
	args := make([][]byte, 0)
	log.Debug("cmd:", common.CmdDescription(cmd), "details:", buf)
	if argc == 0 {
		return nil, true
	}

	if argc == 1 {
		args = append(args, buf)
		return args, true
	}

	for i := 0; i < argc-1 && endPos < len(buf); i++ {
		startPos := endPos
		pos := bytes.IndexByte(buf[startPos:], 0x0)
		endPos = startPos + pos
		args = append(args, buf[startPos:endPos])
		endPos++
	}

	args = append(args, buf[endPos:]) //last one is data

	if len(args) == argc {
		return args, true
	}

	log.Errorf("%d-%d", argc, len(args))

	return nil, false
}

func constructReply(tp uint32, data [][]byte) []byte {
	buf := &bytes.Buffer{}
	err := binary.Write(buf, binary.BigEndian, uint32(common.Res))
	if err != nil {
		panic("should never happend")
	}

	binary.Write(buf, binary.BigEndian, tp)
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

	binary.Write(buf, binary.BigEndian, uint32(length))
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

	log.Warningf("invalid cmd %d", cmd)

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

func ReadMessage(conn net.Conn) (uint32, []byte, error) {
	_, tp, size, err := readHeader(conn)
	if err != nil {
		return 0, nil, err
	}

	buf := make([]byte, size)
	_, err = io.ReadFull(conn, buf)

	return tp, buf, err
}

func readHeader(conn net.Conn) (magic uint32, tp uint32, size uint32, err error) {
	magic, err = readUint32(conn)
	if err != nil {
		return
	}

	if magic != common.Req && magic != common.Res {
		log.Debugf("magic not match %v", magic)
		err = invalidMagic
		return
	}

	tp, err = readUint32(conn)
	if err != nil {
		return
	}

	if !validCmd(tp) {
		//gearman's bug, as protocol, we should treat this an error, but gearman allow it
		if tp == 39 { //wtf: benchmark worker send this
			tp = common.GRAB_JOB_UNIQ
			size, err = readUint32(conn)
			return
		}
		err = invalidArg
		return
	}

	size, err = readUint32(conn)

	return
}

func clearOutbox(outbox chan []byte) {
	for {
		select {
		case _, ok := <-outbox:
			if !ok { //channel is empty
				return
			}
		}
	}
}

func writer(conn net.Conn, outbox chan []byte) {
	defer func() {
		conn.Close()
		clearOutbox(outbox) //incase reader is blocked
	}()

	for {
		select {
		case msg, ok := <-outbox:
			if !ok {
				return
			}
			conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			_, err := conn.Write(msg)
			if err != nil {
				return
			}
		}
	}
}

func readUint32(conn net.Conn) (uint32, error) {
	var value uint32
	err := binary.Read(conn, binary.BigEndian, &value)
	return value, err
}

func ValidProtocolDef() {
	if common.CAN_DO != 1 || common.SUBMIT_JOB_EPOCH != 36 { //protocol check
		panic("protocol define not match")
	}
}

package server

import (
	"bytes"
	"container/list"
	"encoding/binary"
	"errors"
	log "github.com/ngaut/logging"
	"io"
	"net"
	"strconv"
	"time"
)

var (
	invalidMagic = errors.New("invalid magic")
	invalidArg   = errors.New("invalid argument")
)

func cmdDescription(cmd uint32) string {
	if int(cmd) >= len(cmdTable) {
		return "unknown command"
	}
	return cmdTable[cmd].str
}

type cmdstrmap struct {
	cmd  uint32
	str  string
	argc int
}

//stole from libgearman command.cc
var cmdTable = []cmdstrmap{
	{0, "UNUSED", 0},
	{1, "CAN_DO", 1},
	{2, "CANT_DO", 1},
	{3, "RESET_ABILITIES", 0},
	{4, "PRE_SLEEP", 0},
	{5, "UNUSED", 0},
	{6, "NOOP", 0},
	{7, "SUBMIT_JOB", 3},
	{8, "JOB_CREATED", 1},
	{9, "GRAB_JOB", 0},
	{10, "NO_JOB", 0},
	{11, "JOB_ASSIGN", 3},
	{12, "WORK_STATUS", 3},

	{13, "WORK_COMPLETE", 2},

	{14, "WORK_FAIL", 1},

	{15, "GET_STATUS", 1}, //different from libgearman.cc
	{16, "ECHO_REQ", 1},
	{17, "ECHO_RES", 1},
	{18, "SUBMIT_JOB_BG", 3},
	{19, "ERROR", 2},
	{20, "STATUS_RES", 5},
	{21, "SUBMIT_JOB_HIGH", 3},
	{22, "SET_CLIENT_ID", 1},
	{23, "CAN_DO_TIMEOUT", 2},
	{24, "ALL_YOURS", 0},
	{25, "WORK_EXCEPTION", 2},

	{26, "OPTION_REQ", 1},
	{27, "OPTION_RES", 1},
	{28, "WORK_DATA", 2},

	{29, "WORK_WARNING", 2},

	{30, "GRAB_JOB_UNIQ", 0},
	{31, "JOB_ASSIGN_UNIQ", 4},
	{32, "SUBMIT_JOB_HIGH_BG", 3},
	{33, "SUBMIT_JOB_LOW", 3},
	{34, "SUBMIT_JOB_LOW_BG", 3},
	{35, "SUBMIT_JOB_SCHED", 8},
	{36, "SUBMIT_JOB_EPOCH", 4},
}

func decodeArgs(cmd uint32, buf []byte) ([][]byte, bool) {
	argc := cmdTable[cmd].argc
	endPos := 0
	args := make([][]byte, 0)
	log.Debug("cmd:", cmdDescription(cmd), "details:", buf)
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

func constructReply(tp uint32, data [][]byte) ([]byte, error) {
	buf := &bytes.Buffer{}
	err := binary.Write(buf, binary.BigEndian, uint32(res))
	if err != nil {
		return nil, err
	}

	binary.Write(buf, binary.BigEndian, tp)
	if err != nil {
		return nil, err
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
		return nil, err
	}

	for i, arg := range data {
		buf.Write(arg)
		if i < len(data)-1 {
			buf.WriteByte(0x00)
		}
	}

	return buf.Bytes(), err
}

func validCmd(cmd uint32) bool {
	if cmd >= CAN_DO && cmd <= SUBMIT_JOB_EPOCH {
		return true
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

func ReadMessage(conn net.Conn) (uint32, []byte, error) {
	_, tp, size, err := readHeader(conn)
	if err != nil {
		return 0, nil, err
	}

	buf := make([]byte, size)
	n, err := io.ReadFull(conn, buf)
	if uint32(n) != size {
		return 0, nil, err
	}

	return tp, buf, nil
}

func readHeader(conn net.Conn) (magic uint32, tp uint32, size uint32, err error) {
	magic, err = readUint32(conn)
	if err != nil {
		return
	}

	if magic != req && magic != res {
		log.Debugf("magic not match %v", magic)
		err = invalidMagic
		return
	}

	tp, err = readUint32(conn)
	if err != nil {
		return
	}

	if !validCmd(tp) {
		err = invalidArg
		return
	}

	size, err = readUint32(conn)
	if err != nil {
		return
	}

	return
}

type Tuple struct {
	first, second, third, fourth, fifth, sixth interface{}
}

func writer(conn net.Conn, outbox chan []byte) {
	defer conn.Close()
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
	if CAN_DO != 1 || SUBMIT_JOB_EPOCH != 36 { //protocol check
		panic("protocol define not match")
	}
}

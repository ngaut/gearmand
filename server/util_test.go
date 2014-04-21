package server

import (
	"bytes"
	. "github.com/ngaut/gearmand/common"
	"testing"
)

func TestDecodeArgs(t *testing.T) {
	/*
		00 52 45 51                \0REQ        (Magic)
		00 00 00 07                7            (Packet type: SUBMIT_JOB)
		00 00 00 0d                13           (Packet length)
		72 65 76 65 72 73 65 00    reverse\0    (Function)
		00                         \0           (Unique ID)
		74 65 73 74                test         (Workload)
	*/

	data := []byte{
		0x72, 0x65, 0x76, 0x65, 0x72, 0x73, 0x65, 0x00,
		0x00,
		0x74, 0x65, 0x73, 0x74}
	slice, ok := decodeArgs(SUBMIT_JOB, data)
	if !ok {
		t.Error("should be true")
	}

	if len(slice) != 3 {
		t.Error("arg count not match")
	}

	if !bytes.Equal(slice[0], []byte{0x72, 0x65, 0x76, 0x65, 0x72, 0x73, 0x65}) {
		t.Errorf("decode not match %+v", slice)
	}

	if !bytes.Equal(slice[1], []byte{}) {
		t.Error("decode not match")
	}

	if !bytes.Equal(slice[2], []byte{0x74, 0x65, 0x73, 0x74}) {
		t.Error("decode not match")
	}
}

package main

import (
	"strconv"
	"sync/atomic"
	"time"
)

type Job struct {
	handle      string //server job handle
	id          string
	data        []byte
	running     bool
	percent     int
	denominator int
	createAt    time.Time
	processAt   time.Time
	timeoutSec  int
	createBy    int64 //client sessionId
	processBy   int64 //worker sessionId
	FuncName    string
}

var startJid int64 = 0

func allocJobId() string {
	jid := atomic.AddInt64(&startJid, 1)
	return strconv.Itoa(int(jid))
}

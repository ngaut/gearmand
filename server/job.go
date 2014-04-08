package server

import (
	"fmt"
	"os"
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

var workerNameStr string

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

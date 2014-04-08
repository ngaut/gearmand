package main

import (
	"net"
)

const (
	wsRuning = 1
	wsSleep  = 2
)

type Worker struct {
	net.Conn
	sessionId   int64
	workerId    string
	status      int
	runningJobs map[string]*Job
	canDo       map[string]bool

	outbox chan []byte
}

package server

import (
	. "github.com/ngaut/gearmand/common"
	"net"
)

const (
	wsRuning = 1
	wsSleep  = 2
)

type Worker struct {
	net.Conn
	Session

	workerId    string
	status      int
	runningJobs map[string]*Job
	canDo       map[string]bool
}

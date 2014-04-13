package server

import (
	"time"
)

type Session struct {
	SessionId int64
	Outbox    chan []byte
	ConnectAt time.Time
}

type Client struct {
	Session
}

func (self *Session) TrySend(data []byte) bool {
	select {
	case self.Outbox <- data:
		return true
	default:
		return false
	}
}

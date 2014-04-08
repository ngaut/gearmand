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

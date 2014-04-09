package common

import (
	"time"
)

type Job struct {
	Handle      string //server job handle
	Id          string
	Data        []byte
	Running     bool
	Percent     int
	Denominator int
	CreateAt    time.Time
	ProcessAt   time.Time
	TimeoutSec  int
	CreateBy    int64 //client sessionId
	ProcessBy   int64 //worker sessionId
	FuncName    string
}

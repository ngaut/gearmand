package common

import (
	"time"
)

const (
	PRIORITY_LOW  = 0
	PRIORITY_HIGH = 1
)

type Job struct {
	Handle       string //server job handle
	Id           string
	Data         []byte
	Running      bool
	Percent      int
	Denominator  int
	CreateAt     time.Time
	ProcessAt    time.Time
	TimeoutSec   int
	CreateBy     int64 //client sessionId
	ProcessBy    int64 //worker sessionId
	FuncName     string
	IsBackGround bool
	Priority     int
}

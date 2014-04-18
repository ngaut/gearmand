package server

import (
	"testing"
)

func TestTrySend(t *testing.T) {
	c := &Client{Session: Session{Outbox: make(chan []byte)}}
	if c.TrySend(nil) {
		t.Error("should return false")
	}

	c.Outbox = make(chan []byte, 1)
	if !c.TrySend(nil) {
		t.Error("should return true")
	}
}

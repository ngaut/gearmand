package main

type Client struct {
	sessionId int64

	outbox chan []byte
}

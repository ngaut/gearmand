package main

import (
	"flag"
	gearmand "github.com/ngaut/gearmand/server"
	"net/http"
)

func main() {
	flag.Parse()
	gearmand.ValidProtocolDef()
	go gearmand.NewServer().Start()
	http.ListenAndServe(":6666", nil)
}

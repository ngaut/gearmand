package main

import (
	"flag"
	gearmand "github.com/ngaut/gearmand/server"
	log "github.com/ngaut/logging"
	"net/http"
	_ "net/http/pprof"
	"runtime"
)

var addr = flag.String("addr", ":4730", "listening on, such as 0.0.0.0:4730")

func main() {
	flag.Parse()
	log.SetLevelByString("error")
	runtime.GOMAXPROCS(2)
	gearmand.ValidProtocolDef()
	go gearmand.NewServer().Start(*addr)
	log.Error(http.ListenAndServe(":6060", nil))
}

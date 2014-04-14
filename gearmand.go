package main

import (
	"flag"
	gearmand "github.com/ngaut/gearmand/server"
	log "github.com/ngaut/logging"
	"net/http"
	_ "net/http/pprof"
	"runtime"
)

func main() {
	flag.Parse()
	log.SetLevelByString("warning")
	runtime.GOMAXPROCS(2)
	gearmand.ValidProtocolDef()
	go gearmand.NewServer().Start()
	log.Error(http.ListenAndServe(":6060", nil))
}

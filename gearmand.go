package main

import (
	"flag"
	gearmand "github.com/ngaut/gearmand/server"
	"github.com/ngaut/gearmand/storage/redisq"
	log "github.com/ngaut/logging"
	"net/http"
	_ "net/http/pprof"
	"runtime"
)

var (
	addr  = flag.String("addr", ":4730", "listening on, such as 0.0.0.0:4730")
	path  = flag.String("coredump", "./", "coredump file path")
	redis = flag.String("redis", "localhost:6379", "redis address")
)

func main() {
	flag.Parse()
	gearmand.PublishCmdline()
	gearmand.RegisterCoreDump(*path)
	log.SetLevelByString("warning")
	log.SetHighlighting(false)
	runtime.GOMAXPROCS(1)
	go gearmand.NewServer(&redisq.RedisQ{}).Start(*addr)
	log.Error(http.ListenAndServe(":6060", nil))
}

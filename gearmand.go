package main

import (
	"flag"
	gearmand "github.com/ngaut/gearmand/server"
	"github.com/ngaut/gearmand/storage/redisq"
	log "github.com/ngaut/logging"
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
	log.SetLevelByString("debug")
	//log.SetHighlighting(false)
	runtime.GOMAXPROCS(1)
	gearmand.NewServer(&redisq.RedisQ{}).Start(*addr)
}

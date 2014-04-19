package main

import (
	"flag"
	gearmand "github.com/ngaut/gearmand/server"
	"github.com/ngaut/gearmand/storage/mysql"
	"github.com/ngaut/gearmand/storage/redisq"
	log "github.com/ngaut/logging"
	"runtime"
)

var (
	addr  = flag.String("addr", ":4730", "listening on, such as 0.0.0.0:4730")
	path  = flag.String("coredump", "./", "coredump file path")
	redis = flag.String("redis", "localhost:6379", "redis address")
	//todo: read from config files
	mysqlSource = flag.String("mysql", "user:password@tcp(localhost:5555)/gogearmand", "mysql source")
	storage     = flag.String("storage", "mysql", "choose storage(redis or mysql)")
)

func main() {
	flag.Parse()
	gearmand.PublishCmdline()
	gearmand.RegisterCoreDump(*path)
	log.SetLevelByString("debug")
	//log.SetHighlighting(false)
	runtime.GOMAXPROCS(1)
	if *storage == "redis" {
		gearmand.NewServer(&redisq.RedisQ{}).Start(*addr)
	} else if *storage == "mysql" {
		gearmand.NewServer(&mysql.MYSQLStorage{Source: *mysqlSource}).Start(*addr)
	} else {
		log.Error("unknown storage", *storage)
	}
}

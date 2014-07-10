package main

import (
	"flag"
	"fmt"
	gearmand "github.com/ngaut/gearmand/server"
	"github.com/ngaut/gearmand/storage/mysql"
	"github.com/ngaut/gearmand/storage/redisq"
	"github.com/ngaut/gearmand/storage/sqlite3"
	log "github.com/ngaut/logging"
	"runtime"
	"strconv"
)

var (
	addr  = flag.String("addr", ":4730", "listening on, such as 0.0.0.0:4730")
	path  = flag.String("coredump", "./", "coredump file path")
	redis = flag.String("redis", "localhost:6379", "redis address")
	//todo: read from config files
	mysqlSource   = flag.String("mysql", "user:password@tcp(localhost:3306)/gogearmand?parseTime=true", "mysql source")
	sqlite3Source = flag.String("sqlite3", "gearmand.db", "sqlite3 source")
	storage       = flag.String("storage", "mysql", "choose storage(redis or mysql, sqlite3)")
)

func main() {
	flag.Lookup("v").DefValue = fmt.Sprint(log.LOG_LEVEL_WARN)
	flag.Parse()
	gearmand.PublishCmdline()
	gearmand.RegisterCoreDump(*path)
	if lv, err := strconv.Atoi(flag.Lookup("v").Value.String()); err == nil {
		log.SetLevel(log.LogLevel(lv))
	}
	//log.SetHighlighting(false)
	runtime.GOMAXPROCS(1)
	if *storage == "redis" {
		gearmand.NewServer(&redisq.RedisQ{}).Start(*addr)
	} else if *storage == "mysql" {
		gearmand.NewServer(&mysql.MYSQLStorage{Source: *mysqlSource}).Start(*addr)
	} else if *storage == "sqlite3" {
		gearmand.NewServer(&sqlite3.SQLite3Storage{Source: *sqlite3Source}).Start(*addr)
	} else {
		log.Error("unknown storage", *storage)
	}
}

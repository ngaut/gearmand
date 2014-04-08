package main

import (
	"flag"
	"net/http"
)

func main() {
	flag.Parse()
	go NewServer().Start()
	http.ListenAndServe(":6666", nil)
}

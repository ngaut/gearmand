package server

import (
	"github.com/go-martini/martini"
	log "github.com/ngaut/logging"
	"github.com/ngaut/stats"
	"net/http"
	"net/http/pprof"
	_ "net/http/pprof"
	"os"
)

func getJob(s *Server, params martini.Params) string {
	e := &event{tp: ctrlGetJob,
		jobHandle: params["jobhandle"], result: createResCh()}
	s.ctrlEvtCh <- e
	res := <-e.result

	return res.(string)
}

func getWorker(s *Server, params martini.Params) string {
	e := &event{tp: ctrlGetWorker,
		args: &Tuple{t0: params["cando"]}, result: createResCh()}
	s.ctrlEvtCh <- e
	res := <-e.result

	return res.(string)
}

func registerWebHandler(s *Server) {
	addr := os.Getenv("GEARMAND_MONITOR_ADDR")
	if addr == "" {
		addr = ":3000"
	} else if addr == "-" {
		// Don't start web monitor
		return
	}

	m := martini.Classic()

	m.Get("/debug/pprof", pprof.Index)
	m.Get("/debug/pprof/cmdline", pprof.Cmdline)
	m.Get("/debug/pprof/profile", pprof.Profile)
	m.Get("/debug/pprof/symbol", pprof.Symbol)
	m.Post("/debug/pprof/symbol", pprof.Symbol)
	m.Get("/debug/pprof/block", pprof.Handler("block").ServeHTTP)
	m.Get("/debug/pprof/heap", pprof.Handler("heap").ServeHTTP)
	m.Get("/debug/pprof/goroutine", pprof.Handler("goroutine").ServeHTTP)
	m.Get("/debug/pprof/threadcreate", pprof.Handler("threadcreate").ServeHTTP)
	m.Get("/debug/stats", stats.ExpvarHandler)

	m.Get("/job", func(params martini.Params) string {
		return getJob(s, params)
	})

	//get job information using job handle
	m.Get("/job/:jobhandle", func(params martini.Params) string {
		return getJob(s, params)
	})

	m.Get("/worker", func(params martini.Params) string {
		return getWorker(s, params)
	})

	m.Get("/worker/:cando", func(params martini.Params) string {
		return getWorker(s, params)
	})

	log.Fatal(http.ListenAndServe(addr, m))
}

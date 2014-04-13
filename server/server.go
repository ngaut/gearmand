package server

import (
	"bufio"
	"container/list"
	. "github.com/ngaut/gearmand/common"
	log "github.com/ngaut/logging"
	"net"
	"strconv"
	"sync/atomic"
	"time"
)

type Server struct {
	protoEvtCh chan *event
	ctrlEvtCh  chan *event
	funcWorker map[string]*jobworkermap //function worker
	worker     map[int64]*Worker
	client     map[int64]*Client
	jobs       map[string]*Job

	startSessionId int64
}

func NewServer() *Server {
	return &Server{
		funcWorker: make(map[string]*jobworkermap),
		protoEvtCh: make(chan *event, 100),
		ctrlEvtCh:  make(chan *event, 100),
		worker:     make(map[int64]*Worker),
		client:     make(map[int64]*Client),
		jobs:       make(map[string]*Job),
	}

	//todo: load background jobs from storage
}

func (self *Server) Start() {
	ln, err := net.Listen("tcp", ":4730")
	if err != nil {
		log.Fatal(err)
	}

	go self.EvtLoop()

	log.Debug("listening")

	for {
		conn, err := ln.Accept()
		if err != nil {
			// handle error
			continue
		}
		go self.handleConnection(conn)
	}
}

func (self *Server) addWorker(l *list.List, w *Worker) {
	for it := l.Front(); it != nil; it = it.Next() {
		if it.Value.(*Worker).SessionId == w.SessionId {
			log.Warning("already add")
			return
		}
	}

	l.PushBack(w) //add to worker list
}

func (self *Server) removeWorker(l *list.List, sessionId int64) {
	for it := l.Front(); it != nil; it = it.Next() {
		if it.Value.(*Worker).SessionId == sessionId {
			log.Debugf("removeWorker sessionId %d", sessionId)
			l.Remove(it)
			return
		}
	}
}

func (self *Server) removeWorkerBySessionId(sessionId int64) {
	for _, jw := range self.funcWorker {
		self.removeWorker(jw.workers, sessionId)
	}
}

func (self *Server) handleCanDo(funcName string, w *Worker) {
	jw, ok := self.funcWorker[funcName]
	if !ok { //create list
		jw = &jobworkermap{workers: list.New(), jobs: list.New()}
		self.funcWorker[funcName] = jw
	}

	self.addWorker(jw.workers, w)
	self.worker[w.SessionId] = w
}

func (self *Server) addJob(j *Job) {
	jw, ok := self.funcWorker[j.FuncName]
	if !ok { //create list
		jw = &jobworkermap{workers: list.New(), jobs: list.New()}
	}
	jw.jobs.PushBack(j)
}

func (self *Server) doAddJob(j *Job) {
	self.addJob(j)
	self.jobs[j.Handle] = j
	self.wakeupWorker(j.FuncName)
}

func (self *Server) popJob(sessionId int64) (j *Job) {
	for funcName, cando := range self.worker[sessionId].canDo {
		if !cando {
			continue
		}

		if wj, ok := self.funcWorker[funcName]; ok {
			if wj.jobs.Len() == 0 {
				continue
			}

			job := wj.jobs.Front()
			wj.jobs.Remove(job)
			j = job.Value.(*Job)
			return
		}
	}

	return
}

func (self *Server) wakeupWorker(funcName string) {
	if wj, ok := self.funcWorker[funcName]; ok {
		for it := wj.workers.Front(); it != nil; it.Next() {
			w := it.Value.(*Worker)
			if w.status != wsSleep {
				continue
			}

			reply := constructReply(NOOP, nil)
			if !w.TrySend(reply) {
				log.Warningf("worker sessionId %d is full, cap %d", w.SessionId, cap(w.Outbox))
			}

			return
		}
	}
}

func (self *Server) checkAndRemoveJob(tp uint32, j *Job) {
	switch tp {
	case WORK_COMPLETE, WORK_EXCEPTION, WORK_FAIL:
		self.removeJob(j)
	}
}

func (self *Server) removeJob(j *Job) {
	delete(self.jobs, j.Handle)
	delete(self.worker[j.ProcessBy].runningJobs, j.Handle)
}

func (self *Server) handleCtrlEvt(e *event) {
	//args := e.args
	switch e.tp {
	case ctrlCloseSession:
		sessionId := e.fromSessionId
		if w, ok := self.worker[sessionId]; ok {
			if sessionId != w.SessionId {
				log.Fatalf("sessionId not match %d-%d, bug found", sessionId, w.SessionId)
			}
			self.removeWorkerBySessionId(w.SessionId)
			//reschedule these jobs
			for handle, j := range w.runningJobs {
				if handle != j.Handle {
					log.Fatal("handle not match %d-%d", handle, j.Handle)
				}
				self.doAddJob(j)
			}
			delete(self.worker, sessionId)
		}
		if c, ok := self.client[sessionId]; ok {
			log.Debug("removeClient sessionId", sessionId)
			delete(self.client, c.SessionId)
		}
		e.result <- true //notify close finish

	default:
		log.Warningf("%s, %d", CmdDescription(e.tp), e.tp)
	}
}

func (self *Server) handleSubmitJob(e *event) {
	args := e.args
	c := args.t0.(*Client)
	self.client[c.SessionId] = c
	funcName := bytes2str(args.t1)
	j := &Job{Id: bytes2str(args.t2), Data: args.t3.([]byte),
		Handle: allocJobId(), CreateAt: time.Now(), CreateBy: c.SessionId,
		FuncName: funcName, Priority: PRIORITY_LOW}

	switch e.tp {
	case SUBMIT_JOB_LOW_BG, SUBMIT_JOB_HIGH_BG:
		j.IsBackGround = true
		//todo: persistent job
	}

	switch e.tp {
	case SUBMIT_JOB_HIGH, SUBMIT_JOB_HIGH_BG:
		j.Priority = PRIORITY_HIGH
	}

	//log.Debugf("%v, job handle %v, %s", CmdDescription(e.tp), j.Handle, string(j.Data))
	self.doAddJob(j)
	e.result <- j.Handle
}

func (self *Server) handleWorkReport(e *event) {
	args := e.args
	slice := args.t0.([][]byte)
	jobhandle := bytes2str(slice[0])
	sessionId := e.fromSessionId
	j, ok := self.worker[sessionId].runningJobs[jobhandle]

	log.Debugf("%v job handle %v", CmdDescription(e.tp), jobhandle)
	if !ok {
		log.Warningf("job information lost, %v job handle %v, %+v",
			CmdDescription(e.tp), jobhandle, self.jobs)
		return
	}

	if j.Handle != jobhandle {
		log.Fatal("job handle not match")
	}

	if WORK_STATUS == e.tp {
		j.Percent, _ = strconv.Atoi(string(slice[1]))
		j.Denominator, _ = strconv.Atoi(string(slice[2]))
	}

	self.checkAndRemoveJob(e.tp, j)

	//If BackGround, the client is not updated with status or notified when the job has completed (it is detached)
	if j.IsBackGround {
		return
	}

	//notify all clients
	for _, c := range self.client {
		reply := constructReply(e.tp, slice)
		if !c.TrySend(reply) {
			log.Warning("client is full %+v", c)
		}
	}
}

func (self *Server) handleProtoEvt(e *event) {
	args := e.args
	switch e.tp {
	case CAN_DO:
		w := args.t0.(*Worker)
		funcName := args.t1.(string)
		w.canDo[funcName] = true
		self.handleCanDo(funcName, w)
		self.worker[w.SessionId] = w
	case CANT_DO:
		sessionId := e.fromSessionId
		funcName := args.t0.(string)
		if jw, ok := self.funcWorker[funcName]; ok {
			self.removeWorker(jw.workers, sessionId)
		}
		self.worker[sessionId].canDo[funcName] = false
	case SET_CLIENT_ID:
		w := args.t0.(*Worker)
		w.workerId = args.t1.(string)
		log.Debug(w.workerId)
	case CAN_DO_TIMEOUT: //todo: fix timeout support, now just as CAN_DO
		w := args.t0.(*Worker)
		log.Debug("funcName", args.t1)
		self.handleCanDo(args.t1.(string), w)
		self.worker[w.SessionId] = w
	case GRAB_JOB_UNIQ:
		sessionId := e.fromSessionId
		j := self.popJob(sessionId)
		if j != nil {
			j.ProcessAt = time.Now()
			j.ProcessBy = sessionId
			delete(self.jobs, j.Handle)
			//track this job
			w := self.worker[sessionId]
			w.runningJobs[j.Handle] = j
			w.status = wsRuning
		}
		//send job back
		e.result <- j
	case PRE_SLEEP:
		sessionId := args.t0.(int64)
		self.worker[sessionId].status = wsSleep
	case SUBMIT_JOB, SUBMIT_JOB_LOW_BG, SUBMIT_JOB_LOW:
		self.handleSubmitJob(e)
	case GET_STATUS:
		jobhandle := bytes2str(args.t0)
		if job, ok := self.jobs[jobhandle]; ok {
			e.result <- &Tuple{t0: args.t0, t1: true, t2: job.Running,
				t3: job.Percent, t4: job.Denominator}
			break
		}

		e.result <- &Tuple{t0: args.t0, t1: false, t2: false,
			t3: 0, t4: 100}
	case WORK_DATA, WORK_WARNING, WORK_STATUS, WORK_COMPLETE,
		WORK_FAIL, WORK_EXCEPTION:
		self.handleWorkReport(e)
	default:
		log.Warningf("%s, %d", CmdDescription(e.tp), e.tp)
	}
}

func (self *Server) EvtLoop() {
	for {
		select {
		case e := <-self.protoEvtCh:
			self.handleProtoEvt(e)
		case e := <-self.ctrlEvtCh:
			self.handleCtrlEvt(e)
		}
	}
}

func (self *Server) allocSessionId() int64 {
	return atomic.AddInt64(&self.startSessionId, 1)
}

func (self *Server) getWorker(w *Worker, sessionId int64, outch chan []byte, conn net.Conn) *Worker {
	if w != nil {
		return w
	}

	return &Worker{
		Conn: conn, status: wsRuning, Session: Session{SessionId: sessionId,
			Outbox: outch, ConnectAt: time.Now()}, runningJobs: make(map[string]*Job),
		canDo: make(map[string]bool)}
}

func (self *Server) handleConnection(conn net.Conn) {
	sessionId := self.allocSessionId()
	var w *Worker
	var c *Client
	outch := make(chan []byte, 200)
	defer func() {
		e := &event{tp: ctrlCloseSession, fromSessionId: sessionId,
			result: make(chan interface{}, 1)}
		self.ctrlEvtCh <- e
		<-e.result
		close(outch) //notify writer to quit
	}()

	log.Debug("new sessionId", sessionId, "address:", conn.RemoteAddr())

	go writer(conn, outch)

	r := bufio.NewReaderSize(conn, 128*1024)

	for {
		tp, buf, err := ReadMessage(r)
		if err != nil {
			log.Error(err)
			return
		}

		args, ok := decodeArgs(tp, buf)
		if !ok {
			log.Debug("tp:", CmdDescription(tp), "argc not match", "details:", string(buf))
			return
		}

		//log.Debug("tp:", CmdDescription(tp), "len(args):", len(args), "details:", string(buf))

		switch tp {
		case CAN_DO, CAN_DO_TIMEOUT: //todo: CAN_DO_TIMEOUT timeout support
			w = self.getWorker(w, sessionId, outch, conn)
			self.protoEvtCh <- &event{tp: tp, args: &Tuple{
				t0: w,
				t1: string(args[0])}}
		case CANT_DO:
			self.protoEvtCh <- &event{tp: tp, fromSessionId: sessionId,
				args: &Tuple{t0: string(args[0])}}
		case ECHO_REQ:
			reply := constructReply(ECHO_RES, [][]byte{buf})
			outch <- reply
		case PRE_SLEEP:
			self.protoEvtCh <- &event{tp: tp,
				args: &Tuple{t0: sessionId}}
		case SET_CLIENT_ID:
			w = self.getWorker(w, sessionId, outch, conn)
			self.protoEvtCh <- &event{tp: tp, args: &Tuple{t0: w, t1: string(args[0])}}
		case GRAB_JOB_UNIQ:
			e := &event{tp: tp, fromSessionId: sessionId,
				result: make(chan interface{}, 1)}
			self.protoEvtCh <- e
			job := (<-e.result).(*Job)
			if job == nil {
				log.Warning("no job")
				outch <- constructReply(NO_JOB, nil)
				break
			}

			//log.Debugf("%+v", job)
			reply := constructReply(JOB_ASSIGN_UNIQ, [][]byte{
				[]byte(job.Handle), []byte(job.FuncName), []byte(job.Id), job.Data})
			outch <- reply
		case SUBMIT_JOB, SUBMIT_JOB_LOW_BG, SUBMIT_JOB_LOW:
			if c == nil {
				c = &Client{Session: Session{SessionId: sessionId, Outbox: outch, ConnectAt: time.Now()}}
			}
			e := &event{tp: tp,
				args:   &Tuple{t0: c, t1: args[0], t2: args[1], t3: args[2]},
				result: make(chan interface{}, 1),
			}
			self.protoEvtCh <- e
			handle := <-e.result
			reply := constructReply(JOB_CREATED, [][]byte{[]byte(handle.(string))})
			outch <- reply
		case GET_STATUS:
			e := &event{tp: tp, args: &Tuple{t0: args[0]},
				result: make(chan interface{}, 1)}
			self.protoEvtCh <- e

			resultArg := (<-e.result).(*Tuple)
			reply := constructReply(STATUS_RES, [][]byte{resultArg.t0.([]byte),
				bool2bytes(resultArg.t1.(bool)), bool2bytes(resultArg.t2.(bool)),
				int2bytes(resultArg.t3.(int)),
				int2bytes(resultArg.t4.(int))})
			outch <- reply
		case WORK_DATA, WORK_WARNING, WORK_STATUS, WORK_COMPLETE,
			WORK_FAIL, WORK_EXCEPTION:
			//log.Debugf("%s", string(buf))
			self.protoEvtCh <- &event{tp: tp, args: &Tuple{t0: args},
				fromSessionId: sessionId}
		default:
			log.Warningf("not support type %s", CmdDescription(tp))
		}
	}
}

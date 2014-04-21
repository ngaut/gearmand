package server

import (
	"bufio"
	"container/list"
	"encoding/json"
	. "github.com/ngaut/gearmand/common"
	"github.com/ngaut/gearmand/storage"
	log "github.com/ngaut/logging"
	"github.com/ngaut/stats"
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
	opCounter      map[uint32]int64
	store          storage.JobQueue
}

var ( //const replys, to avoid building it every time
	wakeupReply = constructReply(NOOP, nil)
	nojobReply  = constructReply(NO_JOB, nil)
)

func NewServer(store storage.JobQueue) *Server {
	return &Server{
		funcWorker: make(map[string]*jobworkermap),
		protoEvtCh: make(chan *event, 100),
		ctrlEvtCh:  make(chan *event, 100),
		worker:     make(map[int64]*Worker),
		client:     make(map[int64]*Client),
		jobs:       make(map[string]*Job),
		opCounter:  make(map[uint32]int64),
		store:      store,
	}
}

func (self *Server) getAllJobs() {
	jobs, err := self.store.GetJobs()
	if err != nil {
		log.Error(err)
		return
	}

	log.Debugf("%+v", jobs)

	for _, j := range jobs {
		j.ProcessBy = 0 //no body handle it now
		j.CreateBy = 0  //clear
		self.doAddJob(j)
	}
}

func (self *Server) Start(addr string) {
	ln, err := net.Listen("tcp", ":4730")
	if err != nil {
		log.Fatal(err)
	}

	go self.EvtLoop()

	log.Debug("listening on", addr)

	go registerWebHandler(self)

	//load background jobs from storage
	err = self.store.Init()
	if err != nil {
		log.Error(err)
		self.store = nil
	} else {
		self.getAllJobs()
	}

	for {
		conn, err := ln.Accept()
		if err != nil { // handle error
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
	delete(self.worker, sessionId)
}

func (self *Server) handleCanDo(funcName string, w *Worker) {
	w.canDo[funcName] = true
	jw := self.getJobWorkPair(funcName)
	self.addWorker(jw.workers, w)
	self.worker[w.SessionId] = w
}

func (self *Server) getJobWorkPair(funcName string) *jobworkermap {
	jw, ok := self.funcWorker[funcName]
	if !ok { //create list
		jw = &jobworkermap{workers: list.New(), jobs: list.New()}
		self.funcWorker[funcName] = jw
	}

	return jw
}

func (self *Server) addJob(j *Job) {
	jw := self.getJobWorkPair(j.FuncName)
	jw.jobs.PushBack(j)
}

func (self *Server) doAddJob(j *Job) {
	j.ProcessBy = 0
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

func (self *Server) wakeupWorker(funcName string) bool {
	wj, ok := self.funcWorker[funcName]
	if !ok {
		return false
	}

	if wj.jobs.Len() == 0 {
		return false
	}

	for it := wj.workers.Front(); it != nil; it = it.Next() {
		w := it.Value.(*Worker)
		if w.status != wsSleep {
			continue
		}

		log.Debug("wakeup sessionId", w.SessionId)

		if !w.TrySend(wakeupReply) { //todo: queue it maybe
			log.Warningf("worker sessionId %d is full, cap %d", w.SessionId, cap(w.Outbox))
			continue
		}

		return true
	}

	return false
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
	if j.IsBackGround {
		log.Debugf("done job: %v", j.Handle)
		if self.store != nil {
			if err := self.store.DoneJob(j); err != nil {
				log.Warning(err)
			}
		}
	}
}

func (self *Server) handleCloseSession(e *event) error {
	sessionId := e.fromSessionId
	if w, ok := self.worker[sessionId]; ok {
		if sessionId != w.SessionId {
			log.Fatalf("sessionId not match %d-%d, bug found", sessionId, w.SessionId)
		}
		self.removeWorkerBySessionId(w.SessionId)

		//reschedule these jobs, so other workers can handle it
		for handle, j := range w.runningJobs {
			if handle != j.Handle {
				log.Fatal("handle not match %d-%d", handle, j.Handle)
			}
			self.doAddJob(j)
		}
	}
	if c, ok := self.client[sessionId]; ok {
		log.Debug("removeClient sessionId", sessionId)
		delete(self.client, c.SessionId)
	}
	e.result <- true //notify close finish

	return nil
}

func (self *Server) handleGetWorker(e *event) (err error) {
	var buf []byte
	defer func() {
		e.result <- string(buf)
	}()
	cando := e.args.t0.(string)
	log.Debug("get worker", cando)
	if len(cando) == 0 {
		workers := make([]*Worker, 0, len(self.worker))
		for _, v := range self.worker {
			workers = append(workers, v)
		}
		buf, err = json.Marshal(workers)
		if err != nil {
			log.Error(err)
			return err
		}
		return nil
	}

	log.Debugf("%+v", self.funcWorker)

	if jw, ok := self.funcWorker[cando]; ok {
		log.Debug(cando, jw.workers.Len())
		workers := make([]*Worker, 0, jw.workers.Len())
		for it := jw.workers.Front(); it != nil; it = it.Next() {
			workers = append(workers, it.Value.(*Worker))
		}
		buf, err = json.Marshal(workers)
		if err != nil {
			log.Error(err)
			return err
		}
		return nil
	}

	return
}

func (self *Server) handleGetJob(e *event) (err error) {
	log.Debug("get worker", e.jobHandle)
	var buf []byte
	defer func() {
		e.result <- string(buf)
	}()

	if len(e.jobHandle) == 0 {
		buf, err = json.Marshal(self.jobs)
		if err != nil {
			log.Error(err)
			return err
		}
		return nil
	}

	if job, ok := self.jobs[e.jobHandle]; ok {
		buf = []byte(job.String())
		return nil
	}

	return
}

func (self *Server) handleCtrlEvt(e *event) (err error) {
	//args := e.args
	switch e.tp {
	case ctrlCloseSession:
		return self.handleCloseSession(e)
	case ctrlGetJob:
		return self.handleGetJob(e)
	case ctrlGetWorker:
		return self.handleGetWorker(e)
	default:
		log.Warningf("%s, %d", CmdDescription(e.tp), e.tp)
	}

	return nil
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
		// persistent job
		log.Debugf("add job %+v", j)
		if self.store != nil {
			if err := self.store.AddJob(j); err != nil {
				log.Warning(err)
			}
		}
	}

	switch e.tp {
	case SUBMIT_JOB_HIGH, SUBMIT_JOB_HIGH_BG:
		j.Priority = PRIORITY_HIGH
	}

	//log.Debugf("%v, job handle %v, %s", CmdDescription(e.tp), j.Handle, string(j.Data))
	e.result <- j.Handle
	self.doAddJob(j)
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

	//the client is not updated with status or notified when the job has completed (it is detached)
	if j.IsBackGround {
		return
	}

	//broadcast all clients, which is a really bad idea
	//for _, c := range self.client {
	//	reply := constructReply(e.tp, slice)
	//	if !c.TrySend(reply) {
	//		log.Warningf("client is full %+v", c)
	//	}
	//}

	//just send to original client, which is a bad idea too.
	//if need work status notification, you should create co-worker.
	//let worker send status to this co-worker
	c, ok := self.client[j.CreateBy]
	if !ok {
		log.Debug(j.Handle, "sessionId", j.CreateBy, "missing")
		return
	}

	reply := constructReply(e.tp, slice)
	if !c.TrySend(reply) { //it's kind of slow
		log.Warningf("client is full %+v", c)
	}
}

func (self *Server) handleProtoEvt(e *event) {
	args := e.args
	if e.tp < ctrlCloseSession {
		self.opCounter[e.tp]++
	}

	if e.tp >= ctrlCloseSession {
		self.handleCtrlEvt(e)
		return
	}

	switch e.tp {
	case CAN_DO:
		w := args.t0.(*Worker)
		funcName := args.t1.(string)
		self.handleCanDo(funcName, w)
	case CANT_DO:
		sessionId := e.fromSessionId
		funcName := args.t0.(string)
		if jw, ok := self.funcWorker[funcName]; ok {
			self.removeWorker(jw.workers, sessionId)
		}
		delete(self.worker[sessionId].canDo, funcName)
	case SET_CLIENT_ID:
		w := args.t0.(*Worker)
		w.workerId = args.t1.(string)
	case CAN_DO_TIMEOUT: //todo: fix timeout support, now just as CAN_DO
		w := args.t0.(*Worker)
		funcName := args.t1.(string)
		self.handleCanDo(funcName, w)
	case GRAB_JOB_UNIQ:
		sessionId := e.fromSessionId
		w, ok := self.worker[sessionId]
		if !ok {
			log.Fatalf("unregister worker, sessionId %d", sessionId)
			break
		}

		w.status = wsRunning

		j := self.popJob(sessionId)
		if j != nil {
			j.ProcessAt = time.Now()
			j.ProcessBy = sessionId
			//track this job
			w.runningJobs[j.Handle] = j
		} else { //no job
			w.status = wsPrepareForSleep
		}
		//send job back
		e.result <- j
	case PRE_SLEEP:
		sessionId := e.fromSessionId
		w, ok := self.worker[sessionId]
		if !ok {
			log.Warningf("unregister worker, sessionId %d", sessionId)
			w = args.t0.(*Worker)
			self.worker[w.SessionId] = w
			break
		}

		w.status = wsSleep
		log.Debugf("worker sessionId %d sleep", sessionId)
		//check if there is any job for this worker
		for k, _ := range w.canDo {
			if self.wakeupWorker(k) {
				break
			}
		}
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

func (self *Server) wakeupTravel() {
	for k, jw := range self.funcWorker {
		if jw.jobs.Len() > 0 {
			self.wakeupWorker(k)
		}
	}
}

func (self *Server) pubCounter() {
	for k, v := range self.opCounter {
		stats.PubInt64(CmdDescription(k), v)
	}
}

func (self *Server) EvtLoop() {
	tick := time.NewTicker(1 * time.Second)
	for {
		select {
		case e := <-self.protoEvtCh:
			self.handleProtoEvt(e)
		case e := <-self.ctrlEvtCh:
			self.handleCtrlEvt(e)
		case <-tick.C:
			self.pubCounter()
			stats.PubInt("len(protoEvtCh)", len(self.protoEvtCh))
			stats.PubInt("worker count", len(self.worker))
			stats.PubInt("job queue length", len(self.jobs))
			stats.PubInt("queue count", len(self.funcWorker))
			stats.PubInt("client count", len(self.client))
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
		Conn: conn, status: wsSleep, Session: Session{SessionId: sessionId,
			Outbox: outch, ConnectAt: time.Now()}, runningJobs: make(map[string]*Job),
		canDo: make(map[string]bool)}
}

func (self *Server) handleConnection(conn net.Conn) {
	sessionId := self.allocSessionId()
	var w *Worker
	var c *Client
	outch := make(chan []byte, 200)
	defer func() {
		if w != nil || c != nil {
			e := &event{tp: ctrlCloseSession, fromSessionId: sessionId,
				result: createResCh()}
			self.protoEvtCh <- e
			<-e.result
			close(outch) //notify writer to quit
		}
	}()

	log.Debug("new sessionId", sessionId, "address:", conn.RemoteAddr())

	go writer(conn, outch)

	r := bufio.NewReaderSize(conn, 256*1024)
	//todo:1. reuse event's result channel, create less garbage.
	//2. heavily rely on goroutine switch, send reply in EventLoop can make it faster, but logic is not that clean
	//so i am not going to change it right now, maybe never

	for {
		tp, buf, err := ReadMessage(r)
		if err != nil {
			log.Debug(err, "sessionId", sessionId)
			return
		}

		args, ok := decodeArgs(tp, buf)
		if !ok {
			log.Debug("tp:", CmdDescription(tp), "argc not match", "details:", string(buf))
			return
		}

		//log.Debug("sessionId", sessionId, "tp:", CmdDescription(tp), "len(args):", len(args), "details:", string(buf))

		switch tp {
		case CAN_DO, CAN_DO_TIMEOUT: //todo: CAN_DO_TIMEOUT timeout support
			w = self.getWorker(w, sessionId, outch, conn)
			self.protoEvtCh <- &event{tp: tp, args: &Tuple{
				t0: w, t1: string(args[0])}}
		case CANT_DO:
			self.protoEvtCh <- &event{tp: tp, fromSessionId: sessionId,
				args: &Tuple{t0: string(args[0])}}
		case ECHO_REQ:
			sendReply(outch, ECHO_RES, [][]byte{buf})
		case PRE_SLEEP:
			w = self.getWorker(w, sessionId, outch, conn)
			self.protoEvtCh <- &event{tp: tp, args: &Tuple{t0: w}, fromSessionId: sessionId}
		case SET_CLIENT_ID:
			w = self.getWorker(w, sessionId, outch, conn)
			self.protoEvtCh <- &event{tp: tp, args: &Tuple{t0: w, t1: string(args[0])}}
		case GRAB_JOB_UNIQ:
			if w == nil {
				log.Errorf("can't perform GRAB_JOB_UNIQ, need send CAN_DO first")
				return
			}
			e := &event{tp: tp, fromSessionId: sessionId,
				result: createResCh()}
			self.protoEvtCh <- e
			job := (<-e.result).(*Job)
			if job == nil {
				log.Debug("sessionId", sessionId, "no job")
				sendReplyResult(outch, nojobReply)
				break
			}

			//log.Debugf("%+v", job)
			sendReply(outch, JOB_ASSIGN_UNIQ, [][]byte{
				[]byte(job.Handle), []byte(job.FuncName), []byte(job.Id), job.Data})
		case SUBMIT_JOB, SUBMIT_JOB_LOW_BG, SUBMIT_JOB_LOW:
			if c == nil {
				c = &Client{Session: Session{SessionId: sessionId, Outbox: outch,
					ConnectAt: time.Now()}}
			}
			e := &event{tp: tp,
				args:   &Tuple{t0: c, t1: args[0], t2: args[1], t3: args[2]},
				result: createResCh(),
			}
			self.protoEvtCh <- e
			handle := <-e.result
			sendReply(outch, JOB_CREATED, [][]byte{[]byte(handle.(string))})
		case GET_STATUS:
			e := &event{tp: tp, args: &Tuple{t0: args[0]},
				result: createResCh()}
			self.protoEvtCh <- e

			resp := (<-e.result).(*Tuple)
			sendReply(outch, STATUS_RES, [][]byte{resp.t0.([]byte),
				bool2bytes(resp.t1), bool2bytes(resp.t2),
				int2bytes(resp.t3),
				int2bytes(resp.t4)})
		case WORK_DATA, WORK_WARNING, WORK_STATUS, WORK_COMPLETE,
			WORK_FAIL, WORK_EXCEPTION:
			self.protoEvtCh <- &event{tp: tp, args: &Tuple{t0: args},
				fromSessionId: sessionId}
		default:
			log.Warningf("not support type %s", CmdDescription(tp))
		}
	}
}

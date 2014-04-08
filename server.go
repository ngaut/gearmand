package main

import (
	"container/list"
	"encoding/binary"
	log "github.com/ngaut/logging"
	"net"
	"strconv"
	"sync/atomic"
	"time"
)

type Server struct {
	evtch      chan *event
	funcWorker map[string]*jobworkermap //function worker
	worker     map[int64]*Worker
	client     map[int64]*Client
	jobs       map[string]*Job

	startSessionId int64
}

func init() {
	if CAN_DO != 1 || SUBMIT_JOB_EPOCH != 36 { //protocol check
		panic("protocol define not match")
	}
}

func NewServer() *Server {
	return &Server{
		funcWorker: make(map[string]*jobworkermap),
		evtch:      make(chan *event, 100),
		worker:     make(map[int64]*Worker),
		client:     make(map[int64]*Client),
		jobs:       make(map[string]*Job),
	}

	//todo: restore jobs
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
		if it.Value.(*Worker).sessionId == w.sessionId {
			log.Warning("already add")
			return
		}
	}

	l.PushBack(w) //add to worker list
}

func (self *Server) removeWorker(l *list.List, sessionId int64) {
	for it := l.Front(); it != nil; it = it.Next() {
		if it.Value.(*Worker).sessionId == sessionId {
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
	self.worker[w.sessionId] = w
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
	self.jobs[j.handle] = j
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
			reply, err := constructReply(NOOP, nil)
			if err != nil {
				log.Error(err)
				return
			}
			w := it.Value.(*Worker)
			select {
			case w.outbox <- reply:
			default: //todo:maybe this worker is dead
				log.Warningf("worker sessionId %d is full, cap %d", w.sessionId, cap(w.outbox))
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
	delete(self.jobs, j.handle)
	delete(self.worker[j.processBy].runningJobs, j.handle)
}

func (self *Server) EvtLoop() {
	for {
		select {
		case e := <-self.evtch:
			args := e.args
			switch e.tp {
			case CAN_DO:
				w := args.first.(*Worker)
				funcName := args.second.(string)
				w.canDo[funcName] = true
				self.handleCanDo(funcName, w)
				self.worker[w.sessionId] = w
			case CANT_DO:
				sessionId := e.fromSessionId
				funcName := args.first.(string)
				if jw, ok := self.funcWorker[funcName]; ok {
					self.removeWorker(jw.workers, sessionId)
				}
				self.worker[sessionId].canDo[funcName] = false
			case SET_CLIENT_ID:
				w := args.first.(*Worker)
				w.workerId = args.second.(string)
				log.Debug(w.workerId)
			case CAN_DO_TIMEOUT: //todo: fix timeout support, now just as CAN_DO
				w := args.first.(*Worker)
				log.Debug("funcName", args.second)
				self.handleCanDo(args.second.(string), w)
				self.worker[w.sessionId] = w
			case GRAB_JOB_UNIQ:
				sessionId := e.fromSessionId
				j := self.popJob(sessionId)
				if j != nil {
					j.processAt = time.Now()
					j.processBy = sessionId
					delete(self.jobs, j.handle)
					//track this job
					self.worker[sessionId].runningJobs[j.handle] = j
				}
				//send job back
				e.result <- j
			case PRE_SLEEP:
				sessionId := args.first.(int64)
				self.worker[sessionId].status = wsSleep
			case SUBMIT_JOB, SUBMIT_JOB_LOW_BG, SUBMIT_JOB_LOW:
				c := args.first.(*Client)
				self.client[c.sessionId] = c
				funcName := bytes2str(args.second)
				j := &Job{id: bytes2str(args.third), data: args.fourth.([]byte),
					handle: allocJobId(), createAt: time.Now(), createBy: c.sessionId,
					FuncName: funcName}
				log.Debugf("%v, job handle %v, %s", cmdDescription(e.tp), j.handle, string(j.data))
				//todo: persistent job to db
				self.doAddJob(j)
				e.result <- j.handle
			case custormizeClose:
				sessionId := e.fromSessionId
				if w, ok := self.worker[sessionId]; ok {
					if sessionId != w.sessionId {
						log.Fatalf("sessionId not match %d-%d, bug found", sessionId, w.sessionId)
					}
					self.removeWorkerBySessionId(w.sessionId)
					//reschdule these jobs
					for handle, j := range w.runningJobs {
						if handle != j.handle {
							log.Fatal("handle not match %d-%d", handle, j.handle)
						}
						self.doAddJob(j)
					}
					delete(self.worker, sessionId)
				}
				if c, ok := self.client[sessionId]; ok {
					log.Debug("removeClient sessionId", sessionId)
					delete(self.client, c.sessionId)
				}
				e.result <- true //notify close finish
			case GET_STATUS:
				jobhandle := bytes2str(args.first)
				if job, ok := self.jobs[jobhandle]; ok {
					e.result <- &Tuple{first: args.first, second: true, third: job.running,
						fourth: job.percent, fifth: job.denominator}
					break
				}

				e.result <- &Tuple{first: args.first, second: false, third: false,
					fourth: 0, fifth: 100}
			case WORK_DATA, WORK_WARNING, WORK_STATUS, WORK_COMPLETE,
				WORK_FAIL, WORK_EXCEPTION:
				slice := args.first.([][]byte)
				jobhandle := bytes2str(slice[0])
				sessionId := e.fromSessionId
				j, ok := self.worker[sessionId].runningJobs[jobhandle]

				log.Debugf("%v job handle %v", cmdDescription(e.tp), jobhandle)
				if !ok {
					log.Warningf("job information lost, %v job handle %v, %+v",
						cmdDescription(e.tp), jobhandle, self.jobs)
					break
				}

				if j.handle != jobhandle {
					log.Fatal("job handle not match")
				}

				if WORK_STATUS == e.tp {
					j.percent, _ = strconv.Atoi(string(slice[1]))
					j.denominator, _ = strconv.Atoi(string(slice[2]))
				}

				c, ok := self.client[j.createBy]
				if !ok {
					log.Warningf("client information lost, client sessionId", j.createBy)
					break
				}
				reply, err := constructReply(e.tp, slice)
				if err != nil {
					log.Error(err)
				}

				self.checkAndRemoveJob(e.tp, j)

				select {
				case c.outbox <- reply:
				default:
					log.Warning("client is full %+v", c)
				}

			default:
				log.Warningf("%s, %d", cmdDescription(e.tp), e.tp)
			}
		}
	}
}

func readUint32(conn net.Conn) (uint32, error) {
	var value uint32
	err := binary.Read(conn, binary.BigEndian, &value)
	return value, err
}

func (self *Server) allocSessionId() int64 {
	return atomic.AddInt64(&self.startSessionId, 1)
}

func readHeader(conn net.Conn) (magic uint32, tp uint32, size uint32, err error) {
	magic, err = readUint32(conn)
	if err != nil {
		return
	}

	if magic != req && magic != res {
		log.Debugf("magic not match %v", magic)
		err = invalidMagic
		return
	}

	tp, err = readUint32(conn)
	if err != nil {
		return
	}

	if !validCmd(tp) {
		err = invalidArg
		return
	}

	size, err = readUint32(conn)
	if err != nil {
		return
	}

	return
}

type Tuple struct {
	first, second, third, fourth, fifth, sixth interface{}
}

func writer(conn net.Conn, outbox chan []byte) {
	defer conn.Close()
	for {
		select {
		case msg, ok := <-outbox:
			if !ok {
				return
			}
			conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			_, err := conn.Write(msg)
			if err != nil {
				return
			}
		}
	}
}

func (self *Server) handleConnection(conn net.Conn) {
	sessionId := self.allocSessionId()
	var w *Worker
	var c *Client
	outch := make(chan []byte, 200)
	defer func() {

		if sessionId != 0 { //notify close
			e := &event{tp: custormizeClose, fromSessionId: sessionId,
				result: make(chan interface{}, 1)}
			self.evtch <- e
			<-e.result
		}
		close(outch)
	}()

	log.Debug("new sessionId", sessionId, "address:", conn.RemoteAddr())

	go writer(conn, outch)

	for {
		tp, buf, err := ReadMessage(conn)
		if err != nil {
			log.Info(err)
			return
		}

		args, ok := decodeArgs(tp, buf)
		if !ok {
			log.Debug("tp:", cmdDescription(tp), "argc not match", "details:", string(buf))
			return
		}

		log.Debug("tp:", cmdDescription(tp), "len(args):", len(args), "details:", string(buf))

		switch tp {
		case CAN_DO, CAN_DO_TIMEOUT: //todo: CAN_DO_TIMEOUT timeout support
			if w == nil {
				w = &Worker{
					Conn: conn, status: wsRuning, sessionId: sessionId,
					outbox: outch, runningJobs: make(map[string]*Job),
					canDo: make(map[string]bool)}
			}

			self.evtch <- &event{tp: tp, args: &Tuple{
				first:  w,
				second: string(args[0])}}
		case CANT_DO:
			self.evtch <- &event{tp: tp, fromSessionId: sessionId,
				args: &Tuple{first: string(args[0])}}
		case ECHO_REQ:
			if buf == nil {
				log.Error("argument missing")
				return
			}

			reply, err := constructReply(ECHO_RES, [][]byte{buf})
			if err != nil {
				log.Debug(err)
			}
			outch <- reply
		case PRE_SLEEP:
			self.evtch <- &event{tp: tp,
				args: &Tuple{first: sessionId}}
		case SET_CLIENT_ID:
			self.evtch <- &event{tp: tp, args: &Tuple{first: w, second: string(args[0])}}
		case GRAB_JOB_UNIQ:
			e := &event{tp: tp, fromSessionId: sessionId,
				result: make(chan interface{}, 1)}
			self.evtch <- e
			job := (<-e.result).(*Job)
			if job == nil {
				reply, err := constructReply(NO_JOB, nil)
				if err != nil {
					log.Debug(err)
					break
				}
				outch <- reply
				break
			}

			log.Debugf("%+v", job)
			reply, err := constructReply(JOB_ASSIGN_UNIQ, [][]byte{
				[]byte(job.handle), []byte(job.FuncName), []byte(job.id), job.data})
			if err != nil {
				log.Debug(err)
				break
			}
			outch <- reply
		case SUBMIT_JOB, SUBMIT_JOB_LOW_BG, SUBMIT_JOB_LOW: //todo: handle difference
			if c == nil {
				c = &Client{sessionId: sessionId, outbox: outch}
			}
			e := &event{tp: tp,
				args:   &Tuple{first: c, second: args[0], third: args[1], fourth: args[2]},
				result: make(chan interface{}, 1),
			}
			self.evtch <- e
			handle := <-e.result
			reply, err := constructReply(JOB_CREATED, [][]byte{[]byte(handle.(string))})
			if err != nil {
				log.Debug(err)
			}
			outch <- reply
		case GET_STATUS:
			e := &event{tp: tp, args: &Tuple{first: args[0]},
				result: make(chan interface{}, 1)}
			self.evtch <- e

			resultArg := (<-e.result).(*Tuple)

			reply, err := constructReply(STATUS_RES, [][]byte{resultArg.first.([]byte),
				bool2bytes(resultArg.second.(bool)), bool2bytes(resultArg.third.(bool)),
				int2bytes(resultArg.fourth.(int)),
				int2bytes(resultArg.fifth.(int))})
			if err != nil {
				log.Debug(err)
			}
			outch <- reply
		case WORK_DATA, WORK_WARNING, WORK_STATUS, WORK_COMPLETE,
			WORK_FAIL, WORK_EXCEPTION:
			log.Debugf("%s", string(buf))
			self.evtch <- &event{tp: tp, args: &Tuple{first: args},
				fromSessionId: sessionId}
		default:
			log.Warningf("not support type %s", cmdDescription(tp))
		}
	}
}

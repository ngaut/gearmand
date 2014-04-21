package server

import (
	"bufio"
	. "github.com/ngaut/gearmand/common"
	log "github.com/ngaut/logging"
	"net"
	"time"
)

type session struct {
	sessionId int64
	w         *Worker
	c         *Client
}

func (self *session) getWorker(sessionId int64, outch chan []byte, conn net.Conn) *Worker {
	if self.w != nil {
		return self.w
	}

	self.w = &Worker{
		Conn: conn, status: wsSleep, Session: Session{SessionId: sessionId,
			Outbox: outch, ConnectAt: time.Now()}, runningJobs: make(map[string]*Job),
		canDo: make(map[string]bool)}

	return self.w
}

func (self *session) handleConnection(s *Server, conn net.Conn) {
	sessionId := s.allocSessionId()

	outch := make(chan []byte, 200)
	defer func() {
		if self.w != nil || self.c != nil {
			e := &event{tp: ctrlCloseSession, fromSessionId: sessionId,
				result: createResCh()}
			s.protoEvtCh <- e
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

		log.Debug("sessionId", sessionId, "tp:", CmdDescription(tp), "len(args):", len(args), "details:", string(buf))

		switch tp {
		case CAN_DO, CAN_DO_TIMEOUT: //todo: CAN_DO_TIMEOUT timeout support
			self.w = self.getWorker(sessionId, outch, conn)
			s.protoEvtCh <- &event{tp: tp, args: &Tuple{
				t0: self.w, t1: string(args[0])}}
		case CANT_DO:
			s.protoEvtCh <- &event{tp: tp, fromSessionId: sessionId,
				args: &Tuple{t0: string(args[0])}}
		case ECHO_REQ:
			sendReply(outch, ECHO_RES, [][]byte{buf})
		case PRE_SLEEP:
			self.w = self.getWorker(sessionId, outch, conn)
			s.protoEvtCh <- &event{tp: tp, args: &Tuple{t0: self.w}, fromSessionId: sessionId}
		case SET_CLIENT_ID:
			self.w = self.getWorker(sessionId, outch, conn)
			s.protoEvtCh <- &event{tp: tp, args: &Tuple{t0: self.w, t1: string(args[0])}}
		case GRAB_JOB_UNIQ:
			if self.w == nil {
				log.Errorf("can't perform GRAB_JOB_UNIQ, need send CAN_DO first")
				return
			}
			e := &event{tp: tp, fromSessionId: sessionId,
				result: createResCh()}
			s.protoEvtCh <- e
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
			if self.c == nil {
				self.c = &Client{Session: Session{SessionId: sessionId, Outbox: outch,
					ConnectAt: time.Now()}}
			}
			e := &event{tp: tp,
				args:   &Tuple{t0: self.c, t1: args[0], t2: args[1], t3: args[2]},
				result: createResCh(),
			}
			s.protoEvtCh <- e
			handle := <-e.result
			sendReply(outch, JOB_CREATED, [][]byte{[]byte(handle.(string))})
		case GET_STATUS:
			e := &event{tp: tp, args: &Tuple{t0: args[0]},
				result: createResCh()}
			s.protoEvtCh <- e

			resp := (<-e.result).(*Tuple)
			sendReply(outch, STATUS_RES, [][]byte{resp.t0.([]byte),
				bool2bytes(resp.t1), bool2bytes(resp.t2),
				int2bytes(resp.t3),
				int2bytes(resp.t4)})
		case WORK_DATA, WORK_WARNING, WORK_STATUS, WORK_COMPLETE,
			WORK_FAIL, WORK_EXCEPTION:
			s.protoEvtCh <- &event{tp: tp, args: &Tuple{t0: args},
				fromSessionId: sessionId}
		default:
			log.Warningf("not support type %s", CmdDescription(tp))
		}
	}
}

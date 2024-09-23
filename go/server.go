package main

import (
	"context"
	"log"
	"net"
	"time"
)

func serve(listener net.Listener) {
	hctx, hcancel := context.WithCancel(context.Background())
	hj := starthub(hctx, hcancel)

	for {
		rawconn, err := listener.Accept()
		if err != nil {
			log.Printf("failed to accept connection: %v", err)
			continue
		}

		ctx, cancel := context.WithCancel(hctx)
		pc := makeconn(ctx, cancel).start(rawconn, rawconn)
		go pingclient(pc)

		cc := makeclientconn(pc, hj)
		go cc.main()
	}
}

func pingclient(pc protoconn) {
	goinc()
	defer godec()
	tick := time.Tick(pingInterval)
	for {
		select {
		case <-tick:
			pc.send(protomes{t: mping})
		case <-pc.ctx.Done():
			return
		}
	}
}

type roomclienthandle struct {
	ctx    context.Context
	cancel context.CancelFunc
	talk   chan<- string
}

type clientconn struct {
	pc     protoconn
	hj     hubroomjoiner
	rooms  map[uint32]roomclienthandle
	exited chan uint32
}

func makeclientconn(pc protoconn, hj hubroomjoiner) clientconn {
	return clientconn{
		pc:     pc,
		hj:     hj,
		rooms:  make(map[uint32]roomclienthandle),
		exited: make(chan uint32),
	}
}

func (cc clientconn) main() {
	pc := cc.pc
	defer pc.cancel()

	rl := ratelimiter(pc.ctx.Done(), incomingRateLimit, incomingBurstLimit)
	for {
		select {
		case <-pc.ctx.Done():
			return
		case room := <-cc.exited:
			delete(cc.rooms, room)
		case m := <-pc.in:
			cc.handle(m)
		}
		select {
		case <-pc.ctx.Done():
			return
		case <-rl:
		}
	}
}

func (cc clientconn) handle(m protomes) {
	pc := cc.pc

	switch m.t {
	case mjoin:
		cc.handlejoin(m.room, m.name)
	case mexit:
		cc.handleexit(m.room)
	case mtalk:
		cc.handletalk(m.room, m.text)
	case mpong:
	default:
		pc.send(errormes(errInvalidMessageType))
	}
}

func (cc clientconn) handlejoin(room uint32, name string) {
	if _, ok := cc.rooms[room]; ok {
		return
	}
	hj, pc := cc.hj, cc.pc

	// the room does NON-BLOCKING SENDS to these channels
	// so they need some buffer
	// (although the room also throttles itself as to not overwhelm its clients)
	// TODO: ^actually implement that

	rms := make(chan roommes, roomCapacity/2)
	jned := make(chan string, roomCapacity/4)
	exed := make(chan string, roomCapacity/4)

	resp := make(chan roomhandle)
	prob := make(chan zero)

	joined := false

	ctx, cancel := context.WithCancel(pc.ctx)
	defer func() {
		prf("r! joined %v? %v\n", room, joined)
		if !joined {
			cancel()
		}
	}()
	{
		done := pc.ctx.Done()
		exited := cc.exited
		context.AfterFunc(ctx, func() {
			select {
			case <-done:
			case exited <- room:
			}
		})
	}

	req := joinroomreq{
		name: name,
		rms:  rms,
		jned: jned,
		exed: exed,
		done: ctx.Done(),
		resp: resp,
		prob: prob,
	}
	hreq := hjoinroomreq{room, req}

	select {
	case <-hj.done:
	case hj.reqs <- hreq:
		select {
		case <-time.After(roomTimeout):
			// TODO: out <- room unavailable (could turn "errJoinFailed" into this more general room unavailable error)
		case <-prob:
			pc.send(errormes(errJoinFailed))
		case rh := <-resp:
			texts := make(chan string)
			rc := roomclient{
				ctx:    ctx,
				cancel: cancel,
				out:    pc.out,
				rh:     rh,
				name:   name,
				room:   room,
				texts:  texts,
				rms:    rms,
				jned:   jned,
				exed:   exed,
			}
			go rc.main()
			joined = true
			rt := roomclienthandle{
				ctx:    ctx,
				cancel: cancel,
				talk:   texts,
			}
			cc.rooms[room] = rt
		}
	}
}

func (cc clientconn) handleexit(room uint32) {
	rch, ok := cc.rooms[room]
	if !ok {
		return
	}
	rch.cancel()
}

func (cc clientconn) handletalk(room uint32, text string) {
	rch, ok := cc.rooms[room]
	if !ok {
		prf("c! not in room\n")
		return
	}
	select {
	case <-rch.ctx.Done():
		prf("c! room unavailable\n")
		// TODO: room unavailable?
	case rch.talk <- text:
	}
}

type roomclient struct {
	ctx    context.Context
	cancel context.CancelFunc
	out    chan<- protomes
	rh     roomhandle
	name   string
	room   uint32
	texts  <-chan string
	rms    <-chan roommes
	jned   <-chan string
	exed   <-chan string
}

func (rc roomclient) connsend(m protomes) {
	select {
	case <-rc.ctx.Done():
		prf("r! conn done\n")
	case rc.out <- m:
	}
}

func (rc roomclient) roomsend(rm roommes) {
	rh := rc.rh
	select {
	case <-rh.ctx.Done():
		prf("r! room done\n")
		//TODO: out<-room unavailable
	case <-time.After(roomTimeout):
		prf("r! room timeout\n")
		//TODO out<-room unavailable
	case rh.rms <- rm:
	}
}

func (rc roomclient) main() {
	rh := rc.rh

	defer rc.cancel()
	defer rh.cancel()

	for {
		select {
		case <-rc.ctx.Done():
			prf("r! room conn done\n")
			return
		case <-rh.ctx.Done():
			prf("r! room handle done\n")
			return
		case text := <-rc.texts:
			rm := roommes{name: rc.name, text: text}
			prf("r! receiving roommes %v\n", rm)
			rc.roomsend(rm)
		case rm := <-rc.rms:
			m := protomes{t: mhear, room: rc.room, name: rm.name, text: rm.text}
			prf("r! sending hear %v\n", m)
			rc.connsend(m)
		case name := <-rc.jned:
			m := protomes{t: mjned, room: rc.room, name: name}
			prf("r! sending jned %v\n", m)
			rc.connsend(m)
		case name := <-rc.exed:
			m := protomes{t: mexed, room: rc.room, name: name}
			prf("r! sending exed %v\n", m)
			rc.connsend(m)
		}
	}
}

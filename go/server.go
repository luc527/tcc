package main

import (
	"bytes"
	"context"
	"log"
	"net"
	"strconv"
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
		pc := makeconn(ctx, cancel).start(rawconn, nil)
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
	name   string
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

	rl := ratelimiter(pc.ctx.Done(), clientRateLimit, clientBurstLimit)
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
	case mlsro:
		cc.handlelsro()
	case mpong:
	default:
		pc.send(errormes(ebadtype))
	}
}

func (cc clientconn) handlejoin(room uint32, name string) {
	pc := cc.pc
	if _, ok := cc.rooms[room]; ok {
		pc.send(errormes(ejoined))
		return
	}
	if len(cc.rooms) >= clientMaxRooms {
		pc.send(errormes(eroomfull))
		return
	}
	hj := cc.hj

	// the room does NON-BLOCKING SENDS to these channels
	// so they need some buffer
	// (although the room also throttles itself as to not overwhelm its clients)
	// TODO: ^actually implement that

	// TODO: review chan sizes
	rms := make(chan roommes, roomCapacity/2)
	jned := make(chan string, roomCapacity/4)
	exed := make(chan string, roomCapacity/4)

	resp := make(chan roomhandle)
	prob := make(chan zero)

	joined := false

	ctx, cancel := context.WithCancel(pc.ctx)
	defer func() {
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
		case <-prob: // TODO: prob should return some problem id instead of just zero{}
			pc.send(errormes(etransient(mjoin)))
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
				name:   name,
				talk:   texts,
			}
			cc.rooms[room] = rt
		}
	}
}

func (cc clientconn) handleexit(room uint32) {
	rch, ok := cc.rooms[room]
	if !ok {
		cc.pc.send(errormes(ebadroom))
		return
	}
	rch.cancel()
}

func (cc clientconn) handletalk(room uint32, text string) {
	pc := cc.pc
	rch, ok := cc.rooms[room]
	if !ok {
		pc.send(errormes(ebadroom))
		return
	}
	select {
	case <-rch.ctx.Done():
		pc.send(errormes(etransient(mtalk)))
	case rch.talk <- text:
	}
}

func (cc clientconn) handlelsro() {
	bb := new(bytes.Buffer)
	bb.WriteString("room,name\n")
	for room, rch := range cc.rooms {
		bb.WriteString(strconv.FormatUint(uint64(room), 10))
		bb.WriteRune(',')
		bb.WriteString(rch.name)
		bb.WriteRune('\n')
		// the name might contain commas, but since we have only 2 columns, we
		// can assume everything after the first comma is the second column
	}
	cc.pc.send(protomes{t: mrols, text: bb.String()})
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
	case rc.out <- m:
	}
}

func (rc roomclient) roomsend(rm roommes) {
	rh := rc.rh
	select {
	case <-rh.ctx.Done():
		rc.connsend(errormes(etransient(mtalk)))
	case <-time.After(roomTimeout):
		rc.connsend(errormes(etransient(mtalk)))
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
			return
		case <-rh.ctx.Done():
			return
		case text := <-rc.texts:
			rm := roommes{name: rc.name, text: text}
			rc.roomsend(rm)
		case rm := <-rc.rms:
			m := protomes{t: mhear, room: rc.room, name: rm.name, text: rm.text}
			rc.connsend(m)
		case name := <-rc.jned:
			m := protomes{t: mjned, room: rc.room, name: name}
			rc.connsend(m)
		case name := <-rc.exed:
			m := protomes{t: mexed, room: rc.room, name: name}
			rc.connsend(m)
		}
	}
}

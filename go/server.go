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
	// TODO: one hub per core
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

	rl := ratelimiter(pc.ctx.Done(), connIncomingRateLimit, connIncomingBurstLimit)
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
		done: ctx.Done(),
		resp: resp,
		prob: prob,
	}
	hreq := hjoinroomreq{room, req}

	select {
	case <-hj.done:
	case hj.reqs <- hreq:
		select {
		case <-time.After(clientRoomTimeout):
		case <-prob: // TODO: prob should return some problem id instead of just zero{}
			pc.send(errormes(etransient(mjoin)))
		case rh := <-resp:
			texts := make(chan string)
			ch := connhandle{
				ctx:    ctx,
				cancel: cancel,
				out:    pc.out,
				in:     texts,
			}
			rc := roomclient{
				name: name,
				room: room,
				rh:   rh,
				ch:   ch,
				t:    time.NewTimer(0),
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

var (
	etalkdonecc  = etransientprefixed(mtalk, 0x01)
	etalkdonerc  = etransientprefixed(mtalk, 0x02)
	etalktimeout = etransientprefixed(mtalk, 0x03)
)

func (cc clientconn) handletalk(room uint32, text string) {
	pc := cc.pc
	rch, ok := cc.rooms[room]
	if !ok {
		pc.send(errormes(ebadroom))
		return
	}
	select {
	case <-rch.ctx.Done():
		pc.send(errormes(etalkdonecc))
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

type connhandle struct {
	ctx    context.Context
	cancel context.CancelFunc
	in     <-chan string
	out    chan<- protomes
}

func (ch connhandle) send(m protomes) {
	select {
	case <-ch.ctx.Done():
	case ch.out <- m:
	}
}

type roomclient struct {
	rh   roomhandle
	ch   connhandle
	name string
	room uint32
	t    *time.Timer
}

func (rc roomclient) send(rm roommes) {
	rh := rc.rh
	ch := rc.ch

	t := rc.t
	t.Reset(clientRoomTimeout)

	select {
	case <-rh.ctx.Done():
		ch.send(errormes(etalkdonerc))
	case <-t.C:
		ch.send(errormes(etalktimeout))
	case rh.out <- rm:
	}
}

func (rc roomclient) main() {
	ch := rc.ch
	rh := rc.rh

	defer ch.cancel()
	defer rh.cancel()

	for {
		select {
		// from conn to room:
		case <-ch.ctx.Done():
			return
		case text := <-ch.in:
			rm := roommes{name: rc.name, text: text}
			rc.send(rm)
		// from room to conn:
		case <-rh.ctx.Done():
			return
		case rm := <-rh.in:
			m := protomes{t: mhear, room: rc.room, name: rm.name, text: rm.text}
			ch.send(m)
		case name := <-rh.jned:
			m := protomes{t: mjned, room: rc.room, name: name}
			ch.send(m)
		case name := <-rh.exed:
			m := protomes{t: mexed, room: rc.room, name: name}
			ch.send(m)
		}
	}
}

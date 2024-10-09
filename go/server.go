package main

import (
	"bytes"
	"context"
	"log"
	"maps"
	"net"
	"slices"
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
			pc.send(pingmes())
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
	rl     <-chan zero
	rooms  map[uint32]roomclienthandle
	exited chan uint32
}

func makeclientconn(pc protoconn, hj hubroomjoiner) clientconn {
	rl := ratelimiter(pc.ctx.Done(), connIncomingRateLimit, connIncomingBurstLimit)
	return clientconn{
		pc:     pc,
		hj:     hj,
		rl:     rl,
		rooms:  make(map[uint32]roomclienthandle),
		exited: make(chan uint32),
	}
}

func (cc clientconn) wait() bool {
	select {
	case <-cc.pc.ctx.Done():
		return false
	case <-cc.rl:
		return true
	}
}

func (cc clientconn) main() {
	pc := cc.pc
	defer pc.cancel()

	for {
		select {
		case <-pc.ctx.Done():
			return
		case room := <-cc.exited:
			delete(cc.rooms, room)
		case m := <-pc.in:
			cc.handle(m)
		}
	}
}

func (cc clientconn) handle(m protomes) {
	pc := cc.pc

	if m.t.hasname() && !isnamevalid(m.name) {
		pc.send(probmes(ebadname))
		return
	}
	if m.t.hastext() && !ismesvalid(m.text) {
		pc.send(probmes(ebadmes))
	}

	if m.t == mpong {
		return
	}

	if !m.t.valid() {
		pc.send(probmes(ebadtype))
		return
	}

	if !cc.wait() {
		return
	}

	switch m.t {
	case mjoin:
		cc.handlejoin(m.room, m.name)
	case mexit:
		cc.handleexit(m.room)
	case mtalk:
		cc.handletalk(m.room, m.text)
	case mlsro:
		cc.handlelsro()
	}
}

func (cc clientconn) handlejoin(room uint32, name string) {
	pc := cc.pc
	if _, ok := cc.rooms[room]; ok {
		pc.send(probmes(ejoined))
		return
	}
	if len(cc.rooms) >= clientMaxRooms {
		pc.send(probmes(eroomlimit))
		return
	}
	hj := cc.hj

	resp := make(chan roomhandle)
	prob := make(chan joinprob)

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
		case jp := <-prob: // TODO: prob should return some problem id instead of just zero{}
			switch jp {
			case jfull:
				pc.send(probmes(eroomfull))
			case jname:
				pc.send(probmes(enameinuse))
			case jtransient:
				pc.send(probmes(etransient(mjoin)))
			default:
				log.Printf("unknown join prob: %d", jp)
			}
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
		cc.pc.send(probmes(ebadroom))
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
		pc.send(probmes(ebadroom))
		return
	}
	select {
	case <-rch.ctx.Done():
		pc.send(probmes(etalkdonecc))
	case rch.talk <- text:
	}
}

func (cc clientconn) handlelsro() {
	bb := new(bytes.Buffer)
	bb.WriteString("room,name\n")
	rooms := slices.Sorted(maps.Keys(cc.rooms))
	for _, room := range rooms {
		rch := cc.rooms[room]
		bb.WriteString(strconv.FormatUint(uint64(room), 10))
		bb.WriteRune(',')
		bb.WriteString(rch.name)
		bb.WriteRune('\n')
		// the name might contain commas, but since we have only 2 columns, we
		// can assume everything after the first comma is the second column
	}
	cc.pc.send(rolsmes(bb.String()))
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
		ch.send(probmes(etalkdonerc))
	case <-t.C:
		ch.send(probmes(etalktimeout))
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
			m := hearmes(rc.room, rm.name, rm.text)
			ch.send(m)
		case name := <-rh.jned:
			m := jnedmes(rc.room, name)
			ch.send(m)
		case name := <-rh.exed:
			m := exedmes(rc.room, name)
			ch.send(m)
		}
	}
}

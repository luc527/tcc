package main

import (
	"bytes"
	"context"
	"log"
	"maps"
	"net"
	"slices"
	"strconv"
	"tccgo/conn"
	"tccgo/mes"
	"time"
)

// TODO: ensure nodelay is OFF

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
		c := conn.New(ctx, cancel)
		c.Start(rawconn)
		go pingclient(c)

		cc := makeclientconn(ctx, c, hj)
		go cc.main()
	}
}

func pingclient(c conn.Conn) {
	goinc()
	defer godec()
	tick := time.Tick(pingInterval)
	for {
		select {
		case <-tick:
			conn.Send(c, mes.Ping())
		case <-c.Done():
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
	ctx    context.Context
	c      conn.Conn
	hj     hubroomjoiner
	rl     <-chan zero
	rooms  map[uint32]roomclienthandle
	exited chan uint32
}

func makeclientconn(ctx context.Context, c conn.Conn, hj hubroomjoiner) clientconn {
	// rl := ratelimiter(c.Done(), connIncomingRateLimit, connIncomingBurstLimit)
	return clientconn{
		ctx: ctx,
		c:   c,
		hj:  hj,
		// rl:     rl,
		rooms:  make(map[uint32]roomclienthandle),
		exited: make(chan uint32),
	}
}

func (cc clientconn) wait() bool {
	return true

	select {
	case <-cc.c.Done():
		return false
	case <-cc.rl:
		return true
	}
}

func (cc clientconn) main() {
	c := cc.c
	defer c.Stop()

	for {
		select {
		case <-c.Done():
			return
		case room := <-cc.exited:
			delete(cc.rooms, room)
		case m := <-c.In():
			cc.handle(m)
		}
	}
}

func (cc clientconn) handle(m mes.Message) {
	c := cc.c

	if m.T.HasName() && !mes.NameValid(m.Name) {
		conn.Send(c, mes.Prob(mes.ErrBadName))
		return
	}
	if m.T.HasText() && !mes.TextValid(m.Text) {
		conn.Send(c, mes.Prob(mes.ErrBadMessage))
	}

	if m.T == mes.PongType {
		return
	}

	if !m.T.Valid() {
		conn.Send(c, mes.Prob(mes.ErrBadType))
		return
	}

	if !cc.wait() {
		return
	}

	switch m.T {
	case mes.JoinType:
		cc.handlejoin(m.Room, m.Name)
	case mes.ExitType:
		cc.handleexit(m.Room)
	case mes.TalkType:
		cc.handletalk(m.Room, m.Text)
	case mes.LsroType:
		cc.handlelsro()
	}
}

func (cc clientconn) handlejoin(room uint32, name string) {
	c := cc.c
	if _, ok := cc.rooms[room]; ok {
		conn.Send(c, mes.Prob(mes.ErrJoined))
		return
	}
	if len(cc.rooms) >= clientMaxRooms {
		conn.Send(c, mes.Prob(mes.ErrRoomLimit))
		return
	}
	hj := cc.hj

	resp := make(chan roomhandle)
	prob := make(chan joinprob)

	joined := false

	ctx, cancel := context.WithCancel(cc.ctx)
	defer func() {
		if !joined {
			cancel()
		}
	}()
	{
		done := c.Done()
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
		case jp := <-prob:
			switch jp {
			case jfull:
				conn.Send(c, mes.Prob(mes.ErrRoomFull))
			case jname:
				conn.Send(c, mes.Prob(mes.ErrNameInUse))
			case jtransient:
				conn.Send(c, mes.Prob(mes.ErrTransient(mes.JoinType)))
			default:
				log.Printf("unknown join prob: %d", jp)
			}
		case rh := <-resp:
			texts := make(chan string)
			ch := connhandle{
				ctx:    ctx,
				cancel: cancel,
				out:    c.Out(),
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
		conn.Send(cc.c, mes.Prob(mes.ErrBadRoom))
		return
	}
	rch.cancel()
}

var (
	etalkdonecc  = mes.ErrTransientExtra(mes.TalkType, 0x01)
	etalkdonerc  = mes.ErrTransientExtra(mes.TalkType, 0x02)
	etalktimeout = mes.ErrTransientExtra(mes.TalkType, 0x03)
)

func (cc clientconn) handletalk(room uint32, text string) {
	c := cc.c
	rch, ok := cc.rooms[room]
	if !ok {
		conn.Send(c, mes.Prob(mes.ErrBadRoom))
		return
	}
	select {
	case <-rch.ctx.Done():
		conn.Send(c, mes.Prob(etalkdonecc))
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
	conn.Send(cc.c, mes.Rols(bb.String()))
}

type connhandle struct {
	ctx    context.Context
	cancel context.CancelFunc
	in     <-chan string
	out    chan<- mes.Message
}

func (ch connhandle) send(m mes.Message) {
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
		ch.send(mes.Prob(etalkdonerc))
	case <-t.C:
		ch.send(mes.Prob(etalktimeout))
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
			m := mes.Hear(rc.room, rm.name, rm.text)
			ch.send(m)
		case name := <-rh.jned:
			m := mes.Jned(rc.room, name)
			ch.send(m)
		case name := <-rh.exed:
			m := mes.Exed(rc.room, name)
			ch.send(m)
		}
	}
}

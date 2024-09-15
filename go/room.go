package main

import (
	"context"
)

type roommes struct {
	name string
	text string
}

type clienthandle struct {
	ctx   context.Context
	mesc  chan<- roommes
	jnedc chan<- string
	exedc chan<- string
}

type joinroomreq struct {
	name  string
	mesc  chan<- roommes        // channel for the room to send messages to
	jnedc chan<- string         // channel for the room to send joined signals to
	exedc chan<- string         // channel for the room to send exited signals to
	resp  chan<- sender[string] // if the request succeeds, the room sends a room handle to this channel
	prob  chan<- zero           // if the request fails, zero{} is sent to this channel
}

type room struct {
	ctx     context.Context
	cancelf context.CancelFunc
	reqc    chan joinroomreq
	mesc    chan roommes
}

func makeroom(parctx context.Context) room {
	ctx, cancelf := context.WithCancel(parctx)
	reqc := make(chan joinroomreq)
	mesc := make(chan roommes)
	return room{ctx, cancelf, reqc, mesc}
}

func (r room) join(req joinroomreq) {
	select {
	case <-r.ctx.Done():
		req.prob <- zero{}
	case r.reqc <- req:
	}
}

func (r room) send(rm roommes) {
	select {
	case <-r.ctx.Done():
	case r.mesc <- rm:
	}
}

// TODO: when sending to the client, the room maybe shouldn't only check
// whether the client closed. maybe it should also give up on sending a message
// if some time passes. otherwise one slow client could block the room
// (the room sends to the clients one by one)
// ofc using buffered channels for mesc/jnedc/exedc would help a lot, but may not be enough?
// then, what's the timeout? 1 millisecond? idk really, what's the amount that would actually help?
// but also, is it ok to create a timer for each and every message to the client?
// ! that's not necessary, could just reuse the same timer, it could even be one timer per room (maybe?)
// anyhow, with a large enough buffered channel and a decent client connection
// it's very unlikely that we'd even get to the point of losing messages like that

func (r room) main(f roomfreer) {
	defer f.free()
	defer r.cancelf()

	// TODO: check whether double cancelf() is a problem
	// (hub gets canceled -> room gets canceled, but we still call cancelf in a defer)

	clients := make(map[string]clienthandle)

	exitc := make(chan string)
	mesc := make(chan roommes)

	for {
		select {
		case <-r.ctx.Done():
			return
		case req := <-r.reqc:
			if _, exists := clients[req.name]; exists {
				req.prob <- zero{}
				continue
			}
			clictx, clicancelf := context.WithCancel(r.ctx)
			f := freer[string]{
				done: r.ctx.Done(),
				c:    exitc,
				id:   req.name,
			}
			roomser := sender[roommes]{
				done: r.ctx.Done(),
				c:    mesc,
				cf:   clicancelf,
			}
			textser, textrer := senderreceiver(clictx.Done(), make(chan string), clicancelf)
			go handleroomclient(req.name, roomser, textrer, f)

			ch := clienthandle{clictx, req.mesc, req.jnedc, req.exedc}
			clients[req.name] = ch

			req.resp <- textser
			for _, ch := range clients {
				ch.joined(req.name)
			}

		case m := <-mesc:
			for _, ch := range clients {
				ch.send(m)
			}
		case name := <-exitc:
			if _, exists := clients[name]; !exists {
				continue
			}
			for _, ch := range clients {
				ch.exited(name)
			}
			delete(clients, name)
			if len(clients) == 0 {
				return
			}
		}
	}
}

func handleroomclient(name string, s sender[roommes], r receiver[string], f freer[string]) {
	defer f.free()
	defer s.close()
	for {
		if text, ok := r.receive(); ok {
			s.send(roommes{name, text})
		} else {
			break
		}
	}
}

func (ch clienthandle) send(rm roommes) {
	select {
	case <-ch.ctx.Done():
	case ch.mesc <- rm:
	}
}

func (ch clienthandle) joined(name string) {
	select {
	case <-ch.ctx.Done():
	case ch.jnedc <- name:
	}
}

func (ch clienthandle) exited(name string) {
	select {
	case <-ch.ctx.Done():
	case ch.exedc <- name:
	}
}

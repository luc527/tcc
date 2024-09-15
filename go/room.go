package main

import (
	"context"
)

type roommes struct {
	name string
	text string
}

type clienthandle struct {
	ctx  context.Context
	rms  chan<- roommes
	jned chan<- string
	exed chan<- string
}

type joinroomreq struct {
	name string
	rms  chan<- roommes        // channel for the room to send messages to
	jned chan<- string         // channel for the room to send joined signals to
	exed chan<- string         // channel for the room to send exited signals to
	resp chan<- sender[string] // if the request succeeds, the room sends a room handle to this channel
	prob chan<- zero           // if the request fails, zero{} is sent to this channel
}

type room struct {
	ctx    context.Context
	cancel context.CancelFunc
	reqs   chan joinroomreq
	rms    chan roommes
}

func makeroom(parctx context.Context) room {
	ctx, cancel := context.WithCancel(parctx)
	reqs := make(chan joinroomreq)
	rms := make(chan roommes)
	return room{ctx, cancel, reqs, rms}
}

func (r room) join(req joinroomreq) {
	select {
	case <-r.ctx.Done():
		req.prob <- zero{}
	case r.reqs <- req:
	}
}

// TODO: when sending to the client, the room maybe shouldn't only check
// whether the client closed. maybe it should also give up on sending a message
// if some time passes. otherwise one slow client could block the room
// (the room sends to the clients one by one)
// ofc using buffered channels for mesc/jned/exed would help a lot, but may not be enough?
// then, what's the timeout? 1 millisecond? idk really, what's the amount that would actually help?
// but also, is it ok to create a timer for each and every message to the client?
// ! that's not necessary, could just reuse the same timer, it could even be one timer per room (maybe?)
// anyhow, with a large enough buffered channel and a decent client connection
// it's very unlikely that we'd even get to the point of losing messages like that

func (r room) main(f roomfreer) {
	goinc()
	defer godec()
	defer f.free()
	defer r.cancel()

	// TODO: check whether double cancel() is a problem
	// (hub gets canceled -> room gets canceled, but we still call cancel in a defer)

	clients := make(map[string]clienthandle)

	exit := make(chan string)
	rms := make(chan roommes)

	for {
		select {
		case <-r.ctx.Done():
			return
		case req := <-r.reqs:
			if _, exists := clients[req.name]; exists {
				req.prob <- zero{}
				continue
			}
			clictx, clicancel := context.WithCancel(r.ctx)
			f := freer[string]{
				done: r.ctx.Done(),
				c:    exit,
				id:   req.name,
			}
			roomser := sender[roommes]{
				done: r.ctx.Done(),
				c:    rms,
				cf:   clicancel,
			}
			textser, textrer := senderreceiver(clictx.Done(), make(chan string), clicancel)
			go handleroomclient(req.name, roomser, textrer, f)

			ch := clienthandle{clictx, req.rms, req.jned, req.exed}
			clients[req.name] = ch

			req.resp <- textser
			for _, ch := range clients {
				ch.joined(req.name)
			}

		case rm := <-rms:
			for _, ch := range clients {
				ch.send(rm)
			}
		case name := <-exit:
			if _, exists := clients[name]; exists {
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
}

func handleroomclient(name string, s sender[roommes], r receiver[string], f freer[string]) {
	goinc()
	defer godec()
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
	case ch.rms <- rm:
	}
}

func (ch clienthandle) joined(name string) {
	select {
	case <-ch.ctx.Done():
	case ch.jned <- name:
	}
}

func (ch clienthandle) exited(name string) {
	select {
	case <-ch.ctx.Done():
	case ch.exed <- name:
	}
}

package main

import (
	"context"
)

type roommes struct {
	name string
	text string
}

type roomhandle struct {
	ctx    context.Context
	cancel context.CancelFunc
	texts  chan<- string
}

type clienthandle struct {
	ctx  context.Context
	rms  chan<- roommes
	jned chan<- string
	exed chan<- string
}

func (ch clienthandle) send(rm roommes) {
	trysend(ch.rms, rm, ch.ctx.Done())
}

func (ch clienthandle) joined(name string) {
	trysend(ch.jned, name, ch.ctx.Done())
}

func (ch clienthandle) exited(name string) {
	trysend(ch.exed, name, ch.ctx.Done())
}

type joinroomreq struct {
	name string
	rms  chan<- roommes    // channel for the room to send messages to
	jned chan<- string     // channel for the room to send joined signals to
	exed chan<- string     // channel for the room to send exited signals to
	resp chan<- roomhandle // if the request succeeds, the room sends a room handle to this channel
	prob chan<- zero       // if the request fails, zero{} is sent to this channel
}

type room struct {
	ctx    context.Context
	cancel context.CancelFunc
	reqs   chan joinroomreq
	rms    chan roommes
}

func makeroom(ctx context.Context, cancel context.CancelFunc) room {
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

func (r room) main() {
	goinc()
	defer godec()
	defer r.cancel()

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

			name := req.name
			ctx, cancel := context.WithCancel(r.ctx)
			context.AfterFunc(ctx, func() {
				trysend(exit, name, r.ctx.Done())
			})
			texts := make(chan string)
			go handleroomclient(ctx, cancel, name, rms, texts)

			ch := clienthandle{
				ctx:  ctx,
				rms:  req.rms,
				jned: req.jned,
				exed: req.exed,
			}
			clients[req.name] = ch
			req.resp <- roomhandle{
				ctx:    ctx,
				cancel: cancel,
				texts:  texts,
			}
			for _, ch := range clients {
				ch.joined(req.name)
			}
		case rm := <-rms:
			for _, ch := range clients {
				ch.send(rm)
			}
		case name := <-exit:
			// TODO: fix: client not receiving message of their own exit
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

func handleroomclient(
	ctx context.Context,
	cancel context.CancelFunc,
	name string,
	rms chan<- roommes,
	texts <-chan string,
) {
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return
		case text := <-texts:
			rm := roommes{name: name, text: text}
			if !trysend(rms, rm, ctx.Done()) {
				return
			}
		}
	}
}

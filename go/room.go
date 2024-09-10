package main

import (
	"context"
)

// a room handle is what a client uses to send messages to the room
type roomhandle struct {
	ctx    context.Context    // room context, to check whether the room closed
	cancel context.CancelFunc // cancel func to quit the room
	mesc   chan string
}

// a room message is a message the user receives from a room
type roommes struct {
	name string
	text string
}

// a client handle is what the room uses to send messages and joined/exited signals to the client
type clienthandle struct {
	ctx   context.Context // client context, child of room context, to check whether the client quit
	mesc  chan<- roommes
	jnedc chan<- string
	exedc chan<- string
}

// the request the client sends in order to join a room
type joinroomreq struct {
	name  string
	mesc  chan roommes    // channel for the room to send messages in
	jnedc chan string     // channel for the room to send joined signals to
	exedc chan string     // channel for the room to send exited signals to
	resp  chan roomhandle // if the request succeeds, the room sends a room handle to this channel
	prob  chan zero       // if the request fails, zero{} is sent to this channel
}

type room struct {
	reqc   chan joinroomreq
	ctx    context.Context
	cancel context.CancelFunc
}

func startroom() room {
	ctx, cancel := context.WithCancel(context.Background())
	reqc := make(chan joinroomreq)
	r := room{reqc, ctx, cancel}
	go r.run()
	return r
}

func (r room) join(req joinroomreq) {
	select {
	case <-r.ctx.Done():
		req.prob <- zero{}
	case r.reqc <- req:
	}
}

func (r room) run() {
	defer func() {
		select {
		case <-r.ctx.Done():
		default:
			r.cancel()
		}
	}()

	clients := make(map[string]clienthandle)

	mesc := make(chan roommes)
	exitc := make(chan string)

	for {
		select {
		case <-r.ctx.Done():
			return
		case mes := <-mesc:
			for _, ch := range clients {
				select {
				case <-ch.ctx.Done():
				case ch.mesc <- mes:
				}
			}
		case name := <-exitc:
			if _, exists := clients[name]; !exists {
				continue
			}
			delete(clients, name)
			if len(clients) == 0 {
				return
			}
			for _, ch := range clients {
				select {
				case <-ch.ctx.Done():
				case ch.exedc <- name:
				}
			}
		case req := <-r.reqc:
			if _, exists := clients[req.name]; exists {
				req.prob <- zero{}
				continue
			}
			clientctx, clientcancel := context.WithCancel(r.ctx)
			clientc := make(chan string)
			rh := roomhandle{clientctx, clientcancel, clientc}

			clients[req.name] = clienthandle{clientctx, req.mesc, req.jnedc, req.exedc}
			for _, ch := range clients {
				select {
				case <-ch.ctx.Done():
				case ch.jnedc <- req.name:
				}
			}

			go rh.run(req.name, mesc, r.ctx, exitc)
			req.resp <- rh
		}
	}
}

func (rh roomhandle) run(name string, mesc chan<- roommes, parentctx context.Context, exitc chan<- string) {
	defer func() {
		select {
		case <-parentctx.Done():
		case exitc <- name:
		}
	}()
	for {
		select {
		case <-rh.ctx.Done():
			return
		case mes := <-rh.mesc:
			mesc <- roommes{name, mes}
		}
	}
}

func (rh roomhandle) exit() {
	select {
	case <-rh.ctx.Done():
	default:
		rh.cancel()
	}
}

func makeclientc() (mesc chan roommes, jnedc chan string, exedc chan string) {
	mesc, jnedc, exedc = make(chan roommes), make(chan string), make(chan string)
	return
}

func makejoinreq(name string, mesc chan roommes, jnedc chan string, exedc chan string) joinroomreq {
	resp, prob := make(chan roomhandle), make(chan zero)
	return joinroomreq{name, mesc, jnedc, exedc, resp, prob}
}

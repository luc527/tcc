package main

import (
	"context"
)

// a room handle is what a client uses to send messages to the room
type roomhandle struct {
	ctx    context.Context    // room context, to check whether the room closed
	cancel context.CancelFunc // cancel func to quit the room
	mesc   chan<- string
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
	mesc  chan<- roommes    // channel for the room to send messages in
	jnedc chan<- string     // channel for the room to send joined signals to
	exedc chan<- string     // channel for the room to send exited signals to
	resp  chan<- roomhandle // if the request succeeds, the room sends a room handle to this channel
	prob  chan<- zero       // if the request fails, zero{} is sent to this channel
}

type room struct {
	reqc   chan joinroomreq
	ctx    context.Context
	cancel context.CancelFunc
}

func startroom(hubctx context.Context, rid uint32, emptyc chan<- uint32) room {
	ctx, cancel := context.WithCancel(hubctx)
	reqc := make(chan joinroomreq)
	r := room{reqc, ctx, cancel}
	go r.run(hubctx, rid, emptyc)
	return r
}

func (r room) join(req joinroomreq) {
	select {
	case <-r.ctx.Done():
		req.prob <- zero{}
	case r.reqc <- req:
	}
}

func (r room) run(hubctx context.Context, rid uint32, emptyc chan<- uint32) {
	defer func() {
		select {
		case <-r.ctx.Done():
		default:
			r.cancel()
		}
		select {
		case <-hubctx.Done():
		case emptyc <- rid:
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
			go consumeclient(req.name, clientc, mesc, exitc, clientctx, r.ctx)
			req.resp <- rh
		}
	}
}

func consumeclient(
	name string,
	inc <-chan string,
	outc chan<- roommes,
	exitc chan<- string,
	clientctx context.Context,
	roomctx context.Context,
) {
	defer func() {
		select {
		case <-roomctx.Done():
		case exitc <- name:
		}
	}()
	for {
		select {
		case <-clientctx.Done():
			return
		case mes := <-inc:
			outc <- roommes{name, mes}
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

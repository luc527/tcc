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
	rms    chan<- roommes
}

type clienthandle struct {
	ctx  context.Context
	rms  chan<- roommes
	jned chan<- string
	exed chan<- string
}

func (ch clienthandle) talk(rm roommes) {
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
	exs    chan string
	clis   map[string]clienthandle
}

func makeroom(ctx context.Context, cancel context.CancelFunc) room {
	reqs := make(chan joinroomreq, 16)
	exs := make(chan string, 16)
	rms := make(chan roommes, roomCapacity/2)
	clis := make(map[string]clienthandle)
	return room{
		ctx:    ctx,
		cancel: cancel,
		reqs:   reqs,
		rms:    rms,
		exs:    exs,
		clis:   clis,
	}
}

func (r room) join(req joinroomreq) {
	select {
	case <-r.ctx.Done():
		req.prob <- zero{}
	case r.reqs <- req:
	}
}

func (r room) main() {
	goinc()
	defer godec()
	defer r.cancel()

	for {
		select {
		case <-r.ctx.Done():
			return
		case req := <-r.reqs:
			r.handlejoin(req)
		case rm := <-r.rms:
			r.handletalk(rm)
		case name := <-r.exs:
			r.handleexit(name)
		}
	}
}

// TODO: implement cpu messages
// ! they should not block the room main loop,
// instead do the computation in a goroutine that sends back the result in a channel,
// and receive from the channel in the main loop

func (r room) handlejoin(req joinroomreq) {
	if len(r.clis) >= roomCapacity {
		req.prob <- zero{}
		return
	}
	if _, exists := r.clis[req.name]; exists {
		req.prob <- zero{}
		return
	}

	name := req.name
	ctx, cancel := context.WithCancel(r.ctx)
	context.AfterFunc(ctx, func() {
		trysend(r.exs, name, r.ctx.Done())
	})

	ch := clienthandle{
		ctx:  ctx,
		rms:  req.rms,
		jned: req.jned,
		exed: req.exed,
	}
	r.clis[req.name] = ch
	req.resp <- roomhandle{
		ctx:    ctx,
		cancel: cancel,
		rms:    r.rms,
	}
	for _, ch := range r.clis {
		ch.joined(name)
	}
}

func (r room) handleexit(name string) {
	if _, exists := r.clis[name]; !exists {
		return
	}
	delete(r.clis, name)
	if len(r.clis) == 0 {
		return
	}
	for _, ch := range r.clis {
		ch.exited(name)
	}
}

func (r room) handletalk(rm roommes) {
	for _, ch := range r.clis {
		ch.talk(rm)
	}
}

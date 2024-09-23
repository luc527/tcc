package main

import (
	"context"
)

type roommes struct {
	name string
	text string
}

// to send messages to a room, one needs a room handle
type roomhandle struct {
	ctx    context.Context
	cancel context.CancelFunc
	rms    chan<- roommes
}

// request to join a room
type joinroomreq struct {
	name string
	rms  chan<- roommes    // channel for the room to send messages to
	jned chan<- string     // channel for the room to send joined signals to
	exed chan<- string     // channel for the room to send exited signals to
	done <-chan zero       // done channel for the request
	resp chan<- roomhandle // if the request succeeds, the room sends a room handle to this channel
	prob chan<- zero       // if the request fails, zero{} is sent to this channel
}

// to join a room, one needs a roomjoiner
type roomjoiner struct {
	reqs chan joinroomreq
	done <-chan zero
}

type room struct {
	ctx    context.Context
	cancel context.CancelFunc
	reqs   chan joinroomreq
	rms    chan roommes
	exits  chan string
	clis   map[string]clienthandle
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
		case name := <-r.exits:
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
		select {
		case req.prob <- zero{}:
		case <-req.done:
		}
		return
	}
	if _, exists := r.clis[req.name]; exists {
		select {
		case req.prob <- zero{}:
		case <-req.done:
		}
		return
	}

	name := req.name
	ctx, cancel := context.WithCancel(r.ctx)

	rh := roomhandle{
		ctx:    ctx,
		cancel: cancel,
		rms:    r.rms,
	}
	select {
	case req.resp <- rh:
	case <-req.done:
		cancel()
		return
	}

	{
		exits := r.exits
		done := r.ctx.Done()
		context.AfterFunc(ctx, func() {
			select {
			case exits <- name:
			case <-done:
			}
		})
	}

	ch := clienthandle{
		done: ctx.Done(),
		rms:  req.rms,
		jned: req.jned,
		exed: req.exed,
	}
	r.clis[req.name] = ch

	for _, ch := range r.clis {
		select {
		case ch.jned <- name:
		case <-ch.done:
		}
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
		select {
		case ch.exed <- name:
		case <-ch.done:
		}
	}
}

func (r room) handletalk(rm roommes) {
	for _, ch := range r.clis {
		select {
		case ch.rms <- rm:
		case <-ch.done:
		}
	}
}

type clienthandle struct {
	done <-chan zero
	rms  chan<- roommes
	jned chan<- string
	exed chan<- string
}

func startroom(ctx context.Context, cancel context.CancelFunc) roomjoiner {
	rms := make(chan roommes, roomCapacity/2)
	reqs := make(chan joinroomreq, roomCapacity/4)
	exits := make(chan string, roomCapacity/4)
	clis := make(map[string]clienthandle)
	r := room{
		ctx:    ctx,
		cancel: cancel,
		reqs:   reqs,
		rms:    rms,
		exits:  exits,
		clis:   clis,
	}
	go r.main()
	return roomjoiner{
		reqs: r.reqs,
		done: ctx.Done(),
	}
}

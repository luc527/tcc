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

	// TODO: test
	rl := ratelimiter(r.ctx.Done(), roomRateLimit, roomBurstLimit)

	// prf("\n\nro! room started\n")
	// defer prf("\n\nro! room ended\n")

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
			if len(r.clis) == 0 {
				// prf("ro! no more clients\n")
				return
			}
		}
		select {
		case <-r.ctx.Done():
			return
		case <-rl:
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
			// prf("ro! handlejoin prob 1\n")
		case <-req.done:
			// prf("ro! handlejoin done 1\n")
		}
		return
	}
	if _, exists := r.clis[req.name]; exists {
		select {
		case req.prob <- zero{}:
			// prf("ro! handlejoin prob 2\n")
		case <-req.done:
			// prf("ro! handlejoin done 2\n")
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
		// prf("ro! handlejoin resp\n")
	case <-req.done:
		// prf("ro! handlejoin done 3\n")
		cancel()
		return
	}

	{
		exits := r.exits
		done := r.ctx.Done()
		context.AfterFunc(ctx, func() {
			select {
			case exits <- name:
				// prf("ro! cli %v exited\n", req.name)
			case <-done:
				// prf("ro! cli %v failed to exit\n", req.name)
			}
		})
	}

	ch := clienthandle{
		rms:  req.rms,
		jned: req.jned,
		exed: req.exed,
	}
	r.clis[req.name] = ch

	for _, ch := range r.clis {
		select {
		case ch.jned <- name:
			// prf("ro! sent %v jned to %v\n", name, x)
		default:
			// prf("ro! failed to send %v jned to %v\n", name, x)
		}
	}
}

func (r room) handleexit(name string) {
	if _, exists := r.clis[name]; !exists {
		// prf("ro! handleexit(%v) return 1\n", name)
		return
	}
	delete(r.clis, name)
	for _, ch := range r.clis {
		select {
		case ch.exed <- name:
			// prf("ro! sent %v exed to %v\n", name, x)
		default:
			// prf("ro! failed to send %v exed to %v\n", name, x)
		}
	}
}

func (r room) handletalk(rm roommes) {
	for _, ch := range r.clis {
		select {
		case ch.rms <- rm:
			// prf("ro! sent rm %v to %v\n", rm, x)
		default:
			// prf("ro! failed to send rm %v to %v\n", rm, x)
		}
	}
}

// the room does NON-BLOCKING sends to clients
// TODO: is it the room who should create these channels? I'm starting to think so,
// considering the fact that they have to be buffered because of the non-blocking sends and so on,
// these are all concerns of the room.
type clienthandle struct {
	rms  chan<- roommes
	jned chan<- string
	exed chan<- string
}

func startroom(ctx context.Context, cancel context.CancelFunc) roomjoiner {
	// TODO: might be better for the room to actually have SMALLER buffers than the clients
	// e.g. if rms had size 100 and rate limit was 1second, then it would take 100 seconds to cast all "backlogged" messages to the clients
	rms := make(chan roommes, roomBufferSize)
	reqs := make(chan joinroomreq, roomBufferSize/2)
	exits := make(chan string, roomBufferSize/2)
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

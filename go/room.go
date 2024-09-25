package main

import (
	"context"
)

type roommes struct {
	name string
	text string
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

// to send and receive messages to a room, one needs a room handle
type roomhandle struct {
	ctx    context.Context
	cancel context.CancelFunc
	out    chan<- roommes
	in     <-chan roommes
	jned   <-chan string
	exed   <-chan string
}

// request to join a room
type joinroomreq struct {
	name string
	done <-chan zero       // done channel for the request
	resp chan<- roomhandle // if the request succeeds, the room sends a room handle to this channel
	prob chan<- zero       // if the request fails, zero{} is sent to this channel
}

type clienthandle struct {
	out  chan<- roommes
	jned chan<- string
	exed chan<- string
}

func (r room) main() {
	goinc()
	defer godec()
	defer r.cancel()

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

func init() {
	if roomOutgoingBufferSize < 4 {
		// this is specially important for the client since the room will only do non-blocking sends to these channels
		panic("roomOutgoingBufferSize must be >= 4, otherwise jned and exed channels will not be buffered")
	}
}

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

	out := make(chan roommes, roomOutgoingBufferSize)
	jned := make(chan string, roomOutgoingBufferSize/4)
	exed := make(chan string, roomOutgoingBufferSize/4)

	rh := roomhandle{
		ctx:    ctx,
		cancel: cancel,
		out:    r.rms,
		in:     out,
		jned:   jned,
		exed:   exed,
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
		out:  out,
		jned: jned,
		exed: exed,
	}
	r.clis[req.name] = ch

	for _, ch := range r.clis {
		select {
		case ch.jned <- name:
		default:
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
		case ch.out <- rm:
		default:
		}
	}
}

func init() {
	if roomIncomingBufferSize < 4 {
		panic("roomIncomingBufferSize must be >= 4, otherwise reqs and exits chans will not be buffered")
	}
}

func startroom(ctx context.Context, cancel context.CancelFunc) roomjoiner {
	rms := make(chan roommes, roomIncomingBufferSize)
	reqs := make(chan joinroomreq, roomIncomingBufferSize/4)
	exits := make(chan string, roomIncomingBufferSize/4)
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

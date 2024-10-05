package main

import (
	"context"
)

type hjoinroomreq struct {
	room uint32
	req  joinroomreq
}

type hubroomjoiner struct {
	reqs chan hjoinroomreq
	done <-chan zero
}

type hub struct {
	ctx    context.Context
	cancel context.CancelFunc
	reqs   chan hjoinroomreq
}

func (h hub) joiner() hubroomjoiner {
	return hubroomjoiner{
		reqs: h.reqs,
		done: h.ctx.Done(),
	}
}

func (h hub) main() {
	goinc()
	defer godec()
	defer h.cancel()

	roomjoiners := make(map[uint32]roomjoiner)
	emptied := make(chan uint32)

	getrj := func(room uint32) roomjoiner {
		var (
			rj roomjoiner
			ok bool
		)
		if rj, ok = roomjoiners[room]; !ok {
			ctx, cancel := context.WithCancel(h.ctx)
			done := h.ctx.Done()
			context.AfterFunc(ctx, func() {
				select {
				case emptied <- room:
				case <-done:
				}
			})
			rj = startroom(ctx, cancel)
			roomjoiners[room] = rj
		}
		return rj
	}

	for {
		select {
		case <-h.ctx.Done():
			return
		case room := <-emptied:
			delete(roomjoiners, room)
		case hreq := <-h.reqs:
			room, req := hreq.room, hreq.req
			rj := getrj(room)
			select {
			case rj.reqs <- req:
			case <-rj.done:
				req.prob <- jtransient
			}
		}
	}
}

func starthub(ctx context.Context, cancel context.CancelFunc) hubroomjoiner {
	reqs := make(chan hjoinroomreq)
	h := hub{ctx, cancel, reqs}
	go h.main()
	return h.joiner()
}

package main

import (
	"context"
)

type hjoinroomreq struct {
	rid uint32
	req joinroomreq
}

type hub struct {
	ctx      context.Context
	cancelf  context.CancelFunc
	joinreqc chan hjoinroomreq
}

func (h hub) close() {
	select {
	case <-h.ctx.Done():
	default:
		h.cancelf()
	}
}

func starthub(parent context.Context) hub {
	ctx, cancelf := context.WithCancel(parent)
	joinreqc := make(chan hjoinroomreq)
	h := hub{ctx, cancelf, joinreqc}
	go h.run()
	return h
}

func (h hub) join(rid uint32, req joinroomreq) {
	hreq := hjoinroomreq{rid, req}
	select {
	case <-h.ctx.Done():
	case h.joinreqc <- hreq:
	}
}

func (h hub) run() {
	defer h.cancelf()

	emptyc := make(chan uint32)
	rooms := make(map[uint32]room)

	for {
		select {
		case <-h.ctx.Done():
			return
		case rid := <-emptyc:
			delete(rooms, rid)
		case hreq := <-h.joinreqc:
			var r room
			if r0, ok := rooms[hreq.rid]; ok {
				r = r0
			} else {
				r = startroom(h.ctx, hreq.rid, emptyc)
				rooms[hreq.rid] = r
			}
			r.join(hreq.req)
		}
	}
}

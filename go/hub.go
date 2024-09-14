package main

import (
	"context"
)

type hjoinroomreq struct {
	rid uint32
	req joinroomreq
}

type hub struct {
	ctx
	joinreqc chan hjoinroomreq
}

func starthub(parent context.Context) hub {
	reqc := make(chan hjoinroomreq)
	h := hub{makectx(parent), reqc}
	go h.run()
	return h
}

func (h hub) join(rid uint32, req joinroomreq) {
	hreq := hjoinroomreq{rid, req}
	select {
	case <-h.done():
		req.prob <- zero{}
	case h.joinreqc <- hreq:
	}
}

func (h hub) run() {
	defer h.cancel()

	emptyc := make(chan uint32)
	rooms := make(map[uint32]room)

	for {
		select {
		case <-h.done():
			return
		case rid := <-emptyc:
			delete(rooms, rid)
		case hreq := <-h.joinreqc:
			var r room
			if r0, ok := rooms[hreq.rid]; ok {
				r = r0
			} else {
				r = startroom(hreq.rid, h.ctx, emptyc)
				rooms[hreq.rid] = r
			}
			r.join(hreq.req)
		}
	}
}

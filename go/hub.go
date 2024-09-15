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
	cancel   context.CancelFunc
	joinreqs chan hjoinroomreq
}

type roomfreer = freer[uint32]

func makehub() hub {
	ctx, cancel := context.WithCancel(context.Background())
	joinreqs := make(chan hjoinroomreq)
	return hub{ctx, cancel, joinreqs}
}

func (h hub) join(rid uint32, req joinroomreq) {
	hreq := hjoinroomreq{rid, req}
	select {
	case <-h.ctx.Done():
		req.prob <- zero{}
	case h.joinreqs <- hreq:
	}
}

func (h hub) main() {
	goinc()
	defer godec()
	defer h.cancel()

	emptied := make(chan uint32)

	rooms := make(map[uint32]room)

	for {
		select {
		case <-h.ctx.Done():
			return
		case rid := <-emptied:
			delete(rooms, rid)
		case hreq := <-h.joinreqs:
			var r room
			if r0, ok := rooms[hreq.rid]; ok {
				r = r0
			} else {
				rf := roomfreer{
					done: h.ctx.Done(),
					c:    emptied,
					id:   hreq.rid,
				}
				r = makeroom(h.ctx)
				go r.main(rf)
				rooms[hreq.rid] = r
			}
			r.join(hreq.req)
		}
	}
}

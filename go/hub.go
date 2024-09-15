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

type roomfreer = freer[uint32]

func makehub() hub {
	ctx, cancelf := context.WithCancel(context.Background())
	joinreqc := make(chan hjoinroomreq)
	return hub{ctx, cancelf, joinreqc}
}

func (h hub) join(rid uint32, req joinroomreq) {
	hreq := hjoinroomreq{rid, req}
	select {
	case <-h.ctx.Done():
		req.prob <- zero{}
	case h.joinreqc <- hreq:
	}
}

func (h hub) main() {
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
				rf := roomfreer{
					done: h.ctx.Done(),
					c:    emptyc,
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

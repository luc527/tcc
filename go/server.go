package main

import (
	"context"
	"net"
	"time"
)

type roomclient struct {
	ctx    context.Context
	cancel context.CancelFunc
	out    chan<- protomes
	rms    chan roommes
	jned   chan string
	exed   chan string
}

func serve(listener net.Listener) {
	h := makehub()
	go h.main()
	for {
		rawconn, err := listener.Accept()
		if err != nil {
			continue
		}

		ctx, cancel := context.WithCancel(context.Background())
		pc := makeconn(ctx, cancel)
		go pc.producein(rawconn)
		go pc.consumeout(rawconn)
		go pingclient(pc)
		go handlemessages(h, pc)
	}
}

func handlemessages(h hub, pc protoconn) {
	defer pc.cancel()

	rooms := make(map[uint32]roomhandle)
	exited := make(chan uint32)

	for {
		select {
		case room := <-exited:
			delete(rooms, room)
		case m := <-pc.in:
			switch m.t {
			case mping:
				if !pc.send(protomes{t: mpong}) {
					return
				}
			case mjoin:
				if _, joined := rooms[m.room]; joined {
					// TODO: respond with error
					continue
				}
				ctx, cancel := context.WithCancel(pc.ctx)
				rc := makeroomclient(ctx, cancel, pc.out)
				req, resp, prob := rc.makereq(m.name)
				h.join(m.room, req)
				select {
				case <-prob:
					cancel()
					if !pc.send(errormes(errJoinFailed)) {
						return
					}
				case rh := <-resp:
					rooms[m.room] = rh
					context.AfterFunc(ctx, func() {
						trysend(exited, m.room, pc.ctx.Done())
					})
					go rc.main(m.room, rh)
				}

			case mexit:
				if rh, ok := rooms[m.room]; ok {
					rh.cancel()
				}
			case msend:
				if rh, ok := rooms[m.room]; ok {
					trysend(rh.texts, m.text, rh.ctx.Done())
				}
			case mpong:
			default:
				if !pc.send(errormes(errInvalidMessageType)) {
					return
				}
			}
		}
	}
}

func makeroomclient(ctx context.Context, cancel context.CancelFunc, out chan<- protomes) roomclient {
	return roomclient{
		ctx:    ctx,
		cancel: cancel,
		out:    out,
		rms:    make(chan roommes),
		jned:   make(chan string),
		exed:   make(chan string),
	}
}

func (rc roomclient) makereq(name string) (req joinroomreq, resp chan roomhandle, prob chan zero) {
	resp = make(chan roomhandle)
	prob = make(chan zero)
	req = joinroomreq{
		name: name,
		rms:  rc.rms,
		jned: rc.jned,
		exed: rc.exed,
		resp: resp,
		prob: prob,
	}
	return
}

func (rc roomclient) send(m protomes) bool {
	return trysend(rc.out, m, rc.ctx.Done())
}

func (rc roomclient) main(room uint32, rh roomhandle) {
	goinc()
	defer godec()

	defer rc.cancel()
	defer rh.cancel()

	for {
		select {
		case rm := <-rc.rms:
			m := protomes{mrecv, room, rm.name, rm.text}
			if !rc.send(m) {
				return
			}
		case name := <-rc.jned:
			m := protomes{mjned, room, name, ""}
			if !rc.send(m) {
				return
			}
		case name := <-rc.exed:
			m := protomes{mexed, room, name, ""}
			if !rc.send(m) {
				return
			}
		case <-rc.ctx.Done():
			return
		case <-rh.ctx.Done():
			return
		}
	}
}

func pingclient(pc protoconn) {
	goinc()
	defer godec()
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if !pc.send(protomes{t: mping}) {
				return
			}
		case <-pc.ctx.Done():
			return
		}
	}
}

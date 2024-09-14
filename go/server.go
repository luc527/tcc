package main

import (
	"context"
	"net"
	"time"
)

// TODO: also reset ping on successful writes?

const (
	pingInterval = 20 * time.Second
)

type clientconn struct {
	protoconn
	resetpingc chan zero
}

type roomclient struct {
	mesc  chan roommes
	jnedc chan string
	exedc chan string
}

func serve(listener net.Listener) {
	h := starthub(context.Background())
	defer h.cancel()
	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		go handleclient(conn, h)
	}
}

func handleclient(conn net.Conn, h hub) {
	cc := clientconn{
		protoconn: protoconn{
			ctx:  h.makechild(),
			inc:  make(chan protomes),
			outc: make(chan protomes),
		},
		resetpingc: make(chan zero),
	}

	go cc.start(conn)
	go cc.ping()
	cc.main(h)
}

func (cc clientconn) ping() {
	timer := time.NewTimer(pingInterval)
	defer timer.Stop()
	for {
		select {
		case <-cc.resetpingc:
		case <-timer.C:
			m := protomes{t: mping}
			if !cc.trysend(cc.outc, m) {
				return
			}
		case <-cc.done():
			return
		}
		timer.Reset(pingInterval)
	}
}

func (cc clientconn) main(h hub) {
	defer cc.cancel()

	rooms := make(map[uint32]roomhandle)

	for {
		select {
		case m := <-cc.inc:
			// TODO: check if this an ok place to reset ping
			// -- doesn't ignore when there are read errors
			cc.resetping()

			switch m.t {
			case mpong:
			case msend:
				if rh, joined := rooms[m.room]; joined {
					rh.trysend(m.text)
				} else {
					// TODO: respond with error, user hasn't joined the room
					continue
				}
			case mjoin:
				if _, alreadyjoined := rooms[m.room]; alreadyjoined {
					// TODO: respond with error?
					continue
				}

				rc := makeroomclient()
				req, resp, prob := rc.makereq(m.name)

				h.join(m.room, req)

				select {
				case <-prob:
					cc.trysend(cc.outc, errormes(errJoinFailed))
				case rh := <-resp:
					rooms[m.room] = rh
					go rc.main(m.room, cc, rh)
				}
			case mexit:
				if rh, joined := rooms[m.room]; joined {
					rh.cancel()
					delete(rooms, m.room)
				}
			default:
				cc.trysend(cc.outc, errormes(errInvalidMessageType))
			}

		case <-cc.done():
			return
		}
	}
}

func (cc clientconn) resetping() bool {
	select {
	case cc.resetpingc <- zero{}:
		return true
	case <-cc.done():
		return false
	}
}

func makeroomclient() roomclient {
	// TODO: buffered?
	return roomclient{
		mesc:  make(chan roommes),
		jnedc: make(chan string),
		exedc: make(chan string),
	}
}

func (rc roomclient) makereq(name string) (joinroomreq, chan roomhandle, chan zero) {
	resp := make(chan roomhandle)
	prob := make(chan zero)
	return joinroomreq{
		name:  name,
		mesc:  rc.mesc,
		jnedc: rc.jnedc,
		exedc: rc.exedc,
		resp:  resp,
		prob:  prob,
	}, resp, prob
}

func (rc roomclient) main(room uint32, cc clientconn, rh roomhandle) {
	defer rh.cancel()
	for {
		select {
		case rmes := <-rc.mesc:
			m := protomes{mrecv, room, rmes.name, rmes.text}
			cc.trysend(cc.outc, m)
		case name := <-rc.jnedc:
			m := protomes{mjned, room, name, ""}
			cc.trysend(cc.outc, m)
		case name := <-rc.exedc:
			m := protomes{mexed, room, name, ""}
			cc.trysend(cc.outc, m)
		case <-cc.done():
			return
		case <-rh.done():
			return
		}
	}
}

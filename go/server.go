package main

import (
	"context"
	"net"
	"time"
)

const (
	pingInterval = 30 * time.Second
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

func (c clientconn) readincoming(rawconn net.Conn) {
	defer c.cancel()
	for {
		select {
		case <-c.done():
			return
		case c.resetpingc <- zero{}:
		}
		if err := c.readsingle(rawconn); err != nil {
			return
		}
	}
}

func serve(listener net.Listener) {
	h := starthub(context.Background())
	defer h.cancel()
	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		go handle(conn, h)
	}
}

func handle(rawconn net.Conn, h hub) {
	conn := clientconn{
		protoconn:  makeconn(h.makechild()),
		resetpingc: make(chan zero),
	}

	context.AfterFunc(conn.ctx.c, func() { rawconn.Close() })

	go conn.readincoming(rawconn)
	go conn.writeoutgoing(rawconn)

	go conn.ping()
	conn.consume(h)
}

func (c clientconn) consume(h hub) {
	defer c.cancel()

	rooms := make(map[uint32]roomhandle)

	// TODO: make a message so the client can query which rooms they
	// have joined and with which name
	// -- client sends mqury, server responds minfo
	// maybe mstat instead of mqury

	for {
		// verbose svlog.Printf("cection %d status: joined rooms %v", c.id, slices.Collect(maps.Keys(rooms)))
		select {
		case <-c.done():
			return
		case m := <-c.inc:
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
					continue
				}
				cli := makeroomclient()
				req, resp, prob := cli.makereq(m.name)
				h.join(m.room, req)
				select {
				case <-prob:
					// TODO: this might be because the name is in use
					// or might be for some other reason
					// so change prob to be not a chan zero but a chan joinerror
					// or something, where joinerror is either canceled or nameInUse
					c.trysend(c.outc, errormes(errJoinFailed))
				case rh := <-resp:
					rooms[m.room] = rh
					go cli.run(m.room, c, rh)
				}
			case mexit:
				if rh, joined := rooms[m.room]; joined {
					rh.cancel()
					delete(rooms, m.room)
				}
			default:
				c.trysend(c.outc, errormes(errInvalidMessageType))
			}
		}
	}
}

func (c clientconn) ping() {
	timer := time.NewTimer(pingInterval)
	defer timer.Stop()
	for {
		select {
		case <-c.resetpingc:
		case <-c.done():
			return
		case <-timer.C:
			if !c.trysend(c.outc, protomes{t: mping}) {
				return
			}
		}
		timer.Reset(pingInterval)
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

func (cli roomclient) makereq(name string) (joinroomreq, chan roomhandle, chan zero) {
	resp := make(chan roomhandle)
	prob := make(chan zero)
	return joinroomreq{
		name:  name,
		mesc:  cli.mesc,
		jnedc: cli.jnedc,
		exedc: cli.exedc,
		resp:  resp,
		prob:  prob,
	}, resp, prob
}

func (cli roomclient) run(room uint32, conn clientconn, rh roomhandle) {
	defer rh.cancel()
	for {
		select {
		case <-conn.done():
			return
		case <-rh.done():
			return
		case rmes := <-cli.mesc:
			m := protomes{mrecv, room, rmes.name, rmes.text}
			conn.trysend(conn.outc, m)
		case name := <-cli.jnedc:
			m := protomes{mjned, room, name, ""}
			conn.trysend(conn.outc, m)
		case name := <-cli.exedc:
			m := protomes{mexed, room, name, ""}
			conn.trysend(conn.outc, m)
		}
	}
}

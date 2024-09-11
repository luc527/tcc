package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"maps"
	"net"
	"os"
	"slices"
	"sync/atomic"
	"time"
)

const (
	pingInterval = 30 * time.Second
	timeout      = 60 * time.Second
)

type zero = struct{}

// TODO: make it clearer that the "client" and the message (mes) are coupled with the network connection
// maybe rename it to "connection"

// abstraction over the network connection
type connection struct {
	ctx
	id         uint64
	inc        chan mes
	outc       chan mes
	resetpingc chan zero
}

type roomclient struct {
	mesc  chan roommes
	jnedc chan string
	exedc chan string
}

// TODO: remove, or maybe only for debug builds or something
var (
	svlog  = log.New(os.Stderr, "<srv> ", 0)
	nextid = atomic.Uint64{}
)

func init() {
	nextid.Store(0)
}

func (conn connection) trysend(dest chan mes, m mes) bool {
	select {
	case <-conn.done():
		return false
	case dest <- m:
		return true
	}
}

func serve(listener net.Listener) {
	h := starthub(context.Background())
	defer h.cancel()
	for {
		conn, err := listener.Accept()
		if err != nil {
			svlog.Printf("accept error: %v", err)
			continue
		}
		go handle(conn, h)
	}
}

func handle(rawconn net.Conn, h hub) {
	id := nextid.Add(1)

	conn := connection{
		id:         id,
		ctx:        h.makechild(),
		inc:        make(chan mes),
		outc:       make(chan mes),
		resetpingc: make(chan zero),
	}

	context.AfterFunc(conn.ctx.c, func() {
		svlog.Printf("connection %d: closed", id)
		rawconn.Close()
	})

	go conn.ping()
	go conn.readincoming(rawconn)
	go conn.writeoutgoing(rawconn)
	conn.consumeincoming(h)
}

func (conn connection) readincoming(rawconn net.Conn) {
	defer func() {
		conn.cancel()
		svlog.Printf("connection %d: reader stopped", conn.id)
	}()

	deadline := time.Now().Add(timeout)
	if err := rawconn.SetReadDeadline(deadline); err != nil {
		err := fmt.Errorf("connection %d: failed to set (1st) read deadline: %w", conn.id, err)
		svlog.Println(err)
		return
	}

	for {
		m := mes{}
		if _, err := m.ReadFrom(rawconn); err == nil {
			svlog.Printf("connection %d: read %v", conn.id, m)
			if !conn.trysend(conn.inc, m) {
				return
			}
		} else if err == io.EOF {
			svlog.Printf("connection %d: disconnected", conn.id)
			return
		} else if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
			svlog.Printf("connection %d: timed out", conn.id)
			return
		} else if merr, ok := err.(merror); ok {
			if !conn.trysend(conn.outc, errormes(merr)) {
				return
			}
		} else {
			err := fmt.Errorf("connection %d: failed to read, partial %v: %w", conn.id, m, err)
			svlog.Println(err)
			return
		}

		conn.resetping()
		deadline := time.Now().Add(timeout)
		if err := rawconn.SetReadDeadline(deadline); err != nil {
			err := fmt.Errorf("connection %d: failed to set read deadline: %w", conn.id, err)
			svlog.Println(err)
			return
		}

	}
}

func (conn connection) writeoutgoing(w io.Writer) {
	defer func() {
		conn.cancel()
		svlog.Printf("connection %d: writer stopped", conn.id)
	}()
	for {
		select {
		case <-conn.done():
			return
		case m := <-conn.outc:
			if _, err := m.WriteTo(w); err == nil {
				svlog.Printf("connection %d: wrote %v", conn.id, m)
			} else {
				err := fmt.Errorf("connection %d: failed to write %v: %w", conn.id, m, err)
				svlog.Println(err)
				return
			}
		}
	}
}

func (conn connection) consumeincoming(h hub) {
	defer conn.cancel()

	rooms := make(map[uint32]roomhandle)

	// TODO: make a message so the client can query which rooms they
	// have joined and with which name
	// -- client sends mqury, server responds minfo
	// maybe mstat instead of mqury

	for {
		svlog.Printf("connection %d status: joined rooms %v", conn.id, slices.Collect(maps.Keys(rooms)))
		select {
		case <-conn.done():
			return
		case m := <-conn.inc:
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
					conn.trysend(conn.outc, errormes(errJoinFailed))
				case rh := <-resp:
					rooms[m.room] = rh
					go cli.run(m.room, conn, rh)
				}
			case mexit:
				if rh, joined := rooms[m.room]; joined {
					rh.cancel()
					delete(rooms, m.room)
				}
			default:
				conn.trysend(conn.outc, errormes(errInvalidMessageType))
			}
		}
	}
}

func (conn connection) resetping() {
	select {
	case <-conn.done():
	case conn.resetpingc <- zero{}:
	}
}

func (conn connection) ping() {
	timer := time.NewTimer(pingInterval)
	defer func() {
		timer.Stop()
		svlog.Printf("connection %d: ping stopped", conn.id)
	}()
	for {
		select {
		case <-conn.resetpingc:
		case <-conn.done():
			return
		case <-timer.C:
			if !conn.trysend(conn.outc, mes{t: mping}) {
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

func (cli roomclient) run(room uint32, conn connection, rh roomhandle) {
	defer rh.cancel()
	for {
		select {
		case <-conn.done():
			return
		case <-rh.done():
			return
		case rmes := <-cli.mesc:
			m := mes{mrecv, room, rmes.name, rmes.text}
			conn.trysend(conn.outc, m)
		case name := <-cli.jnedc:
			m := mes{mjned, room, name, ""}
			conn.trysend(conn.outc, m)
		case name := <-cli.exedc:
			m := mes{mexed, room, name, ""}
			conn.trysend(conn.outc, m)
		}
	}
}

package main

import (
	"net"
)

type serverconn struct {
	protoconn
}

type consumefunc func(protomes, bool)

func (c serverconn) readincoming(rawconn net.Conn) {
	defer c.cancel()
	for {
		if err := c.readsingle(rawconn); err != nil {
			return
		}
	}
}

func (c serverconn) join(room uint32, name string) bool {
	m := protomes{t: mjoin, room: room, name: name}
	return c.trysend(c.outc, m)
}

func (c serverconn) exit(room uint32) bool {
	m := protomes{t: mexit, room: room}
	return c.trysend(c.outc, m)
}

func (c serverconn) send(room uint32, text string) bool {
	m := protomes{t: msend, room: room, text: text}
	return c.trysend(c.outc, m)
}

func (c serverconn) consume(f consumefunc) {
	defer func() {
		c.cancel()
		f(protomes{}, false)
	}()
	for {
		select {
		case <-c.done():
			return
		case m := <-c.inc:
			if m.t == mping {
				m := protomes{t: mpong}
				c.trysend(c.outc, m)
			}
			f(m, true)
		}
	}
}

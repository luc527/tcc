package main

import (
	"net"
)

type serverconn struct {
	protoconn
}

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

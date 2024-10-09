package conn

import (
	"net"
	"tccgo/mes"
)

type Middleware func(mes.Message)

type fconn struct {
	conn
	f    Middleware
	fin  chan mes.Message
	fout chan mes.Message
}

func (c fconn) In() <-chan mes.Message  { return c.fin }
func (c fconn) Out() chan<- mes.Message { return c.fout }

func (c fconn) Start(raw net.Conn) {
	c.conn.Start(raw)
	go runMiddleware(c.conn.out, c.fout, c.ctx.Done(), c.f)
	go runMiddleware(c.fin, c.conn.in, c.ctx.Done(), c.f)
}

func runMiddleware(dest chan<- mes.Message, src <-chan mes.Message, done <-chan zero, f Middleware) {
	for {
		select {
		case v := <-src:
			f(v)
			select {
			case dest <- v:
			case <-done:
				return
			}
		case <-done:
			return
		}
	}
}

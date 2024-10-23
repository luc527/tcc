package conn

import (
	"fmt"
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
	go runMiddleware("out", c.conn.out, c.fout, c.ctx.Done(), c.f)
	go runMiddleware("in", c.fin, c.conn.in, c.ctx.Done(), c.f)
}

func runMiddleware(s string, dest chan<- mes.Message, src <-chan mes.Message, done <-chan zero, f Middleware) {
	for {
		select {
		case v := <-src:
			fmt.Printf("mw %v: before f, %v\n", s, v)
			f(v)
			fmt.Printf("mw %v: after f\n", s)
			select {
			case dest <- v:
				fmt.Printf("mw %v: chan sent\n", s)
			case <-done:
				fmt.Printf("mw %v: chan done\n", s)
				return
			}
		case <-done:
			fmt.Printf("mw %v: chan done outer\n", s)
			return
		}
	}
}

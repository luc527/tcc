package conn

import (
	"context"
	"iter"
	"net"
	"tccgo/mes"
)

type zero = struct{}

type Conn interface {
	Start(net.Conn)
	Close()
	Done() <-chan zero
	In() <-chan mes.Message
	Out() chan<- mes.Message
}

func New(ctx context.Context, cancel context.CancelFunc) Conn {
	return conn{
		ctx:    ctx,
		cancel: cancel,
		in:     make(chan mes.Message),
		out:    make(chan mes.Message),
	}
}

func WithMiddleware(c0 Conn, f Middleware) Conn {
	switch c := c0.(type) {
	case conn:
		return fconn{
			conn: c,
			f:    f,
			fin:  make(chan mes.Message),
			fout: make(chan mes.Message),
		}
	case fconn:
		f0 := c.f
		f := func(m mes.Message) {
			f0(m)
			f(m)
		}
		return fconn{
			conn: c.conn,
			f:    f,
			fin:  c.fin,
			fout: c.fout,
		}
	default:
		panic("unreachable")
	}
}

func Send(c Conn, m mes.Message) bool {
	select {
	case <-c.Done():
		return false
	case c.Out() <- m:
		return true
	}
}

func Messages(c Conn) iter.Seq[mes.Message] {
	return func(yield func(mes.Message) bool) {
		for {
			select {
			case <-c.Done():
				return
			case m := <-c.In():
				if !yield(m) {
					return
				}
			}
		}
	}
}

func Closed(c Conn) bool {
	select {
	case <-c.Done():
		return true
	default:
		return false
	}
}

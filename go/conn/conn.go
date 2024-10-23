package conn

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"syscall"
	"tccgo/mes"
	"time"
)

const (
	ReadTimeout  = 30 * time.Second
	WriteTimeout = 10 * time.Second
)

type conn struct {
	ctx    context.Context
	cancel context.CancelFunc
	in     chan mes.Message
	out    chan mes.Message
}

func (c conn) Stop()                   { c.cancel() }
func (c conn) Done() <-chan zero       { return c.ctx.Done() }
func (c conn) In() <-chan mes.Message  { return c.in }
func (c conn) Out() chan<- mes.Message { return c.out }

func (c conn) Start(raw net.Conn) {
	go c.consumeOut(raw)
	go c.produceIn(raw)
}

func (c conn) produceIn(raw net.Conn) {
	defer c.cancel()

	for {
		deadline := time.Now().Add(ReadTimeout)
		if err := raw.SetReadDeadline(deadline); err != nil {
			return
		}

		m := mes.Message{}
		if _, err := m.ReadFrom(raw); err == nil {
			select {
			case <-c.ctx.Done():
				return
			case c.in <- m:
			}
		} else if e, ok := err.(mes.Error); ok {
			if !Send(c, mes.Prob(e)) {
				return
			}
		} else if err == io.EOF {
			return
		} else if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
			return
		} else {
			ok := errors.Is(err, net.ErrClosed) ||
				// ^ same side that closed the connection tried to read from it (?)
				errors.Is(err, syscall.ECONNRESET)
				// ^ seems to happen when the client side has lots of connections opens, is writing to them, and suddenly closes them
			if !ok {
				log.Printf("unexpected read error: %v", err)
			}
			return
		}
	}
}

func (c conn) consumeOut(raw net.Conn) {
	defer c.cancel()
	defer func() {
		if err := raw.Close(); err != nil {
			log.Printf("failed to close: %v", err)
		}
	}()

	for {
		select {
		case <-c.ctx.Done():
			return
		case m := <-c.out:
			if err := raw.SetWriteDeadline(time.Now().Add(WriteTimeout)); err != nil {
				log.Printf("failed to set write deadline: %v\n", err)
				return
			}
			if _, err := m.WriteTo(raw); err != nil {
				// log.Printf("failed to write: %v\n", err)
				return
			}
		}
	}

}

var _ Conn = conn{}

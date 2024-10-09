package main

import (
	"context"
	"errors"
	"io"
	"iter"
	"log"
	"net"
	"syscall"
	"time"
)

type protoconn struct {
	ctx    context.Context
	cancel context.CancelFunc
	in     chan protomes
	out    chan protomes
}

type dir uint8

const (
	din = dir(iota)
	dout
)

type middleware func(protomes, dir)

func makeconn(ctx context.Context, cancel context.CancelFunc) protoconn {
	in := make(chan protomes)
	out := make(chan protomes, connOutgoingBufferSize)
	return protoconn{
		ctx:    ctx,
		cancel: cancel,
		in:     in,
		out:    out,
	}
}

func (pc protoconn) start(rawconn net.Conn, customcloser io.Closer) protoconn {
	var closer io.Closer = rawconn
	if customcloser != nil {
		closer = customcloser
	}
	go pc.producein(rawconn)
	go pc.consumeout(rawconn, closer)
	return pc
}

func (pc protoconn) send(m protomes) bool {
	select {
	case <-pc.ctx.Done():
		return false
	case pc.out <- m:
		return true
	}
}

func (pc protoconn) producein(rawconn net.Conn) {
	goinc()
	defer godec()
	defer pc.cancel()

	for {
		deadline := time.Now().Add(connReadTimeout)
		if err := rawconn.SetReadDeadline(deadline); err != nil {
			prf("co! failed to set read deadline: %v\n", err)
			return
		}

		m := protomes{}
		if _, err := m.ReadFrom(rawconn); err == nil {
			select {
			case <-pc.ctx.Done():
				return
			case pc.in <- m:
			}
		} else if e, ok := err.(ecode); ok {
			if !pc.send(probmes(e)) {
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

func (pc protoconn) consumeout(rawconn net.Conn, c io.Closer) {
	goinc()
	defer godec()

	defer pc.cancel()
	defer func() {
		if err := c.Close(); err != nil {
			log.Printf("failed to close: %v", err)
		}
	}()

	for {
		select {
		case <-pc.ctx.Done():
			return
		case m := <-pc.out:
			if err := rawconn.SetWriteDeadline(time.Now().Add(connWriteTimeout)); err != nil {
				prf("co! failed to set write deadline: %v\n", err)
				return
			}
			if _, err := m.WriteTo(rawconn); err != nil {
				prf("co! failed to write: %v\n", err)
				return
			}
		}
	}
}

func (pc protoconn) withmiddleware(f middleware) protoconn {
	fpc := pc
	fpc.in = make(chan protomes)
	fpc.out = make(chan protomes)
	go runmiddleware(fpc.in, pc.in, pc.ctx.Done(), din, f)
	go runmiddleware(pc.out, fpc.out, pc.ctx.Done(), dout, f)
	return fpc
}

func (pc protoconn) messages() iter.Seq[protomes] {
	return func(yield func(protomes) bool) {
		for {
			select {
			case <-pc.ctx.Done():
				return
			case m := <-pc.in:
				if !yield(m) {
					return
				}
			}
		}
	}
}

func (pc protoconn) join(room uint32, name string) bool {
	return pc.send(joinmes(room, name))
}

func (pc protoconn) jned(room uint32, name string) bool {
	return pc.send(jnedmes(room, name))
}

func (pc protoconn) exit(room uint32) bool {
	return pc.send(exitmes(room))
}

func (pc protoconn) exed(room uint32, name string) bool {
	return pc.send(exedmes(room, name))
}

func (pc protoconn) talk(room uint32, text string) bool {
	return pc.send(talkmes(room, text))
}

func (pc protoconn) hear(room uint32, name string, text string) bool {
	return pc.send(hearmes(room, name, text))
}

func (pc protoconn) lsro() bool {
	return pc.send(lsromes())
}

func (pc protoconn) rols(csv string) bool {
	return pc.send(rolsmes(csv))
}

func (pc protoconn) ping() bool {
	return pc.send(pingmes())
}

func (pc protoconn) pong() bool {
	return pc.send(pongmes())
}

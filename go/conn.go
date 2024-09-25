package main

import (
	"context"
	"errors"
	"io"
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

type middleware func(protomes)

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

func (pc protoconn) send(m protomes) {
	select {
	case <-pc.ctx.Done():
	case pc.out <- m:
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
			select {
			case <-pc.ctx.Done():
				return
			case pc.out <- errormes(e):
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
	go runmiddleware(fpc.in, pc.in, pc.ctx.Done(), f)
	go runmiddleware(pc.out, fpc.out, pc.ctx.Done(), f)
	return fpc
}

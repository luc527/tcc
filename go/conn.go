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

const (
	timeout      = 30 * time.Second
	pingInterval = 20 * time.Second
)

func init() {
	if pingInterval >= timeout {
		panic("must have pingInterval < timeout")
		// otherwise the server will ping but the timeout will close the connection before the client even gets the
	}
}

type protoconn struct {
	ctx    context.Context
	cancel context.CancelFunc
	in     chan protomes
	out    chan protomes
}

func makeconn(ctx context.Context, cancel context.CancelFunc) protoconn {
	in := make(chan protomes)
	out := make(chan protomes)
	return protoconn{
		ctx:    ctx,
		cancel: cancel,
		in:     in,
		out:    out,
	}
}

func (pc protoconn) start(rawconn net.Conn) protoconn {
	go pc.producein(rawconn)
	go pc.consumeout(rawconn)
	return pc
}

func (pc protoconn) send(m protomes) bool {
	return trysend(pc.out, m, pc.ctx.Done())
}

func (pc protoconn) isdone() bool {
	select {
	case <-pc.ctx.Done():
		return true
	default:
		return false
	}
}

func (pc protoconn) producein(rawconn net.Conn) {
	goinc()
	defer godec()
	defer pc.cancel()
	defer close(pc.in)
	for {
		deadline := time.Now().Add(timeout)
		if err := rawconn.SetReadDeadline(deadline); err != nil {
			return
		}

		m := protomes{}
		if _, err := m.ReadFrom(rawconn); err == nil {
			select {
			case <-pc.ctx.Done():
				return
			case pc.in <- m:
			}
		} else if perr, ok := err.(protoerror); ok {
			select {
			case <-pc.ctx.Done():
				return
			case pc.out <- errormes(perr):
			}
		} else if err == io.EOF {
			return
		} else if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
			return
		} else {
			ok := errors.Is(err, net.ErrClosed) ||
				// ^ same side that closed the connection tried to read from it (?)
				errors.Is(err, syscall.ECONNRESET)
				// ^ seems to happen when the client side has lots of connections opens, is writing to them, and closes them all suddenly
				// TODO: check if disconnecting the bots in the console gradually helps to avoid this, although idk if it even really is a problem
			if !ok {
				log.Printf("unexpected read error: %v", err)
			}
			return
		}

	}
}

func (pc protoconn) consumeout(rawconn net.Conn) {
	goinc()
	defer godec()
	defer pc.cancel()
	defer func() {
		log.Printf("closing")
		if err := rawconn.Close(); err != nil {
			log.Printf("failed to close: %v", err)
		}
	}()
	for {
		select {
		case <-pc.ctx.Done():
			return
		case m := <-pc.out:
			if _, err := m.WriteTo(rawconn); err != nil {
				return
			}
		}
	}
}

// needs to be called before starting the underlying protoconn
func (pc protoconn) startmiddleware(inf func(protomes), outf func(protomes)) protoconn {
	fpc := pc
	fpc.in = make(chan protomes)
	fpc.out = make(chan protomes)
	go runmiddleware(fpc.in, pc.in, pc.ctx.Done(), inf)
	go runmiddleware(pc.out, fpc.out, pc.ctx.Done(), outf)
	return fpc
}

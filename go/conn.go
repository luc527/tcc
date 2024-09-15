package main

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
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
	ctx     context.Context
	cancel  context.CancelFunc
	rawconn net.Conn
	in      chan protomes
	out     chan protomes
}

func makeconn(rawconn net.Conn) protoconn {
	ctx, cancel := context.WithCancel(context.Background())
	in := make(chan protomes)
	out := make(chan protomes)
	return protoconn{
		ctx:     ctx,
		cancel:  cancel,
		rawconn: rawconn,
		in:      in,
		out:     out,
	}
}

func (pc protoconn) producein() {
	goinc()
	defer godec()
	defer pc.cancel()
	for {
		deadline := time.Now().Add(timeout)
		if err := pc.rawconn.SetReadDeadline(deadline); err != nil {
			return
		}

		m := protomes{}
		if _, err := m.ReadFrom(pc.rawconn); err == nil {
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
		} else if errors.Is(err, net.ErrClosed) {
			return
		} else {
			log.Printf("failed to read message: %v", err)
			return
		}
	}
}

func (pc protoconn) consumeout() {
	goinc()
	defer godec()
	defer pc.cancel()
	for {
		select {
		case <-pc.ctx.Done():
			return
		case m := <-pc.out:
			if _, err := m.WriteTo(pc.rawconn); err != nil {
				return
			}
		}
	}
}

func (pc protoconn) sender() sender[protomes] {
	return sender[protomes]{
		done: pc.ctx.Done(),
		c:    pc.out,
		cf: func() {
			if err := pc.rawconn.Close(); err != nil {
				log.Printf("failed to close: %v", err)
			}
		},
	}
}

func (pc protoconn) receiver() receiver[protomes] {
	return receiver[protomes]{
		done: pc.ctx.Done(),
		c:    pc.in,
	}
}

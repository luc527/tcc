package main

import (
	"context"
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
		panic("ping interval has to be lesser than the timeout")
		// otherwise the server will ping but the timeout will close the connection before the client even gets the
	}
}

type protoconn struct {
	ctx     context.Context
	cancelf context.CancelFunc
	rawconn net.Conn
	inc     chan protomes
	outc    chan protomes
}

func makeconn(rawconn net.Conn) protoconn {
	ctx, cancelf := context.WithCancel(context.Background())
	inc := make(chan protomes)
	outc := make(chan protomes)
	return protoconn{
		ctx:     ctx,
		cancelf: cancelf,
		rawconn: rawconn,
		inc:     inc,
		outc:    outc,
	}
}

func (pc protoconn) sender() sender[protomes] {
	return sender[protomes]{
		done: pc.ctx.Done(),
		c:    pc.outc,
		cf: func() {
			if err := pc.rawconn.Close(); err != nil {
				log.Printf("failed to close: %v", err)
			}
		},
	}
}

func (pc protoconn) produceinc() {
	defer pc.cancelf()
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
			case pc.inc <- m:
			}
		} else if perr, ok := err.(protoerror); ok {
			select {
			case <-pc.ctx.Done():
				return
			case pc.outc <- errormes(perr):
			}
		} else if err == io.EOF {
			return
		} else if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
			return
		} else {
			log.Printf("failed to read message: %v", err)
			return
		}
	}
}

func (pc protoconn) consumeoutc() {
	defer pc.cancelf()
	for {
		select {
		case <-pc.ctx.Done():
			return
		case m := <-pc.outc:
			if _, err := m.WriteTo(pc.rawconn); err != nil {
				return
			}
		}
	}
}

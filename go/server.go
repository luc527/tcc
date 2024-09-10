package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync/atomic"
	"time"
)

const (
	pingInterval = 30 * time.Second
	timeout      = 60 * time.Second
)

type zero = struct{}

type client struct {
	id         uint64
	ctx        context.Context
	cancelf    context.CancelFunc
	in         chan mes
	out        chan mes
	resetpingc chan zero
}

// TODO remove
// or maybe only for debug builds or something
var (
	svlog  = log.New(os.Stderr, "<srv> ", 0)
	nextid = atomic.Uint64{}
)

func init() {
	nextid.Store(0)
}

func (c client) cancel() {
	select {
	case <-c.ctx.Done():
	default:
		c.cancelf()
	}
}

func (c client) trysend(dest chan mes, m mes) bool {
	select {
	case <-c.ctx.Done():
		return false
	case dest <- m:
		return true
	}
}

func handle(conn net.Conn) {
	id := nextid.Add(1)

	ctx, cancelf := context.WithCancel(context.Background())
	cli := client{
		id:         id,
		ctx:        ctx,
		cancelf:    cancelf,
		in:         make(chan mes),
		out:        make(chan mes),
		resetpingc: make(chan zero),
	}

	context.AfterFunc(cli.ctx, func() {
		svlog.Printf("client %d: closed", id)
		conn.Close()
	})

	go cli.ping()
	go cli.readincoming(conn)
	go cli.writeoutgoing(conn)
	cli.consumeincoming()
}

func (c client) readincoming(conn net.Conn) {
	defer func() {
		c.cancel()
		close(c.in)
		svlog.Printf("client %d: reader stopped", c.id)
	}()

	deadline := time.Now().Add(timeout)
	if err := conn.SetReadDeadline(deadline); err != nil {
		err := fmt.Errorf("client %d: failed to set (1st) read deadline: %w", c.id, err)
		svlog.Println(err)
		return
	}

	for {
		m := mes{}
		if _, err := m.ReadFrom(conn); err == nil {
			svlog.Printf("client %d: read %v", c.id, m)
			if !c.trysend(c.in, m) {
				return
			}
		} else if err == io.EOF {
			svlog.Printf("client %d: disconnected", c.id)
			return
		} else if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
			svlog.Printf("client %d: timed out", c.id)
			return
		} else if merr, ok := err.(merror); ok {
			if !c.trysend(c.out, errormes(merr)) {
				return
			}
		} else {
			err := fmt.Errorf("client %d: failed to read, partial %v: %w", c.id, m, err)
			svlog.Println(err)
			return
		}

		c.resetping()

		deadline := time.Now().Add(timeout)
		if err := conn.SetReadDeadline(deadline); err != nil {
			err := fmt.Errorf("client %d: failed to set read deadline: %w", c.id, err)
			svlog.Println(err)
			return
		}

	}
}

func (c client) writeoutgoing(w io.Writer) {
	defer func() {
		c.cancel()
		svlog.Printf("client %d: writer stopped", c.id)
	}()
	for {
		select {
		case <-c.ctx.Done():
			return
		case m := <-c.out:
			if _, err := m.WriteTo(w); err == nil {
				svlog.Printf("client %d: wrote %v", c.id, m)
			} else {
				err := fmt.Errorf("client %d: failed to write %v: %w", c.id, m, err)
				svlog.Println(err)
				return
			}
		}
	}
}

func (c client) consumeincoming() {
	// TODO
	for m := range c.in {
		switch m.t {
		case mpong:
		case mjoin:
		case mexit:
		case msend:
		default:
			c.trysend(c.out, errormes(errInvalidMessageType))
		}
	}
}

func (c client) resetping() {
	select {
	case <-c.ctx.Done():
	case c.resetpingc <- zero{}:
	}
}

func (c client) ping() {
	timer := time.NewTimer(pingInterval)
	defer func() {
		timer.Stop()
		svlog.Printf("client %d: ping stopped", c.id)
	}()
	for {
		select {
		case <-c.resetpingc:
		case <-c.ctx.Done():
			return
		case <-timer.C:
			if !c.trysend(c.out, mes{t: mping}) {
				return
			}
		}
		timer.Reset(pingInterval)
	}
}

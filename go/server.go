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
	ctx
	id         uint64
	inc        chan mes
	outc       chan mes
	resetpingc chan zero
}

// TODO: remove, or maybe only for debug builds or something
var (
	svlog  = log.New(os.Stderr, "<srv> ", 0)
	nextid = atomic.Uint64{}
)

func init() {
	nextid.Store(0)
}

func (cli client) trysend(dest chan mes, m mes) bool {
	select {
	case <-cli.done():
		return false
	case dest <- m:
		return true
	}
}

func serve(listener net.Listener) {
	h := starthub(context.Background())
	defer h.cancel()
	for {
		conn, err := listener.Accept()
		if err != nil {
			svlog.Printf("accept error: %v", err)
			continue
		}
		go handle(conn, h)
	}
}

func handle(conn net.Conn, h hub) {
	id := nextid.Add(1)

	cli := client{
		id:         id,
		ctx:        makectx(context.Background()),
		inc:        make(chan mes),
		outc:       make(chan mes),
		resetpingc: make(chan zero),
	}

	context.AfterFunc(cli.ctx.c, func() {
		svlog.Printf("client %d: closed", id)
		conn.Close()
	})

	go cli.ping()
	go cli.readincoming(conn)
	go cli.writeoutgoing(conn)
	cli.consumeincoming(h)
}

func (cli client) readincoming(conn net.Conn) {
	defer func() {
		cli.cancel()
		svlog.Printf("client %d: reader stopped", cli.id)
	}()

	deadline := time.Now().Add(timeout)
	if err := conn.SetReadDeadline(deadline); err != nil {
		err := fmt.Errorf("client %d: failed to set (1st) read deadline: %w", cli.id, err)
		svlog.Println(err)
		return
	}

	for {
		m := mes{}
		if _, err := m.ReadFrom(conn); err == nil {
			svlog.Printf("client %d: read %v", cli.id, m)
			if !cli.trysend(cli.inc, m) {
				return
			}
		} else if err == io.EOF {
			svlog.Printf("client %d: disconnected", cli.id)
			return
		} else if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
			svlog.Printf("client %d: timed out", cli.id)
			return
		} else if merr, ok := err.(merror); ok {
			if !cli.trysend(cli.outc, errormes(merr)) {
				return
			}
		} else {
			err := fmt.Errorf("client %d: failed to read, partial %v: %w", cli.id, m, err)
			svlog.Println(err)
			return
		}

		cli.resetping()
		deadline := time.Now().Add(timeout)
		if err := conn.SetReadDeadline(deadline); err != nil {
			err := fmt.Errorf("client %d: failed to set read deadline: %w", cli.id, err)
			svlog.Println(err)
			return
		}

	}
}

func (cli client) writeoutgoing(w io.Writer) {
	defer func() {
		cli.cancel()
		svlog.Printf("client %d: writer stopped", cli.id)
	}()
	for {
		select {
		case <-cli.done():
			return
		case m := <-cli.outc:
			if _, err := m.WriteTo(w); err == nil {
				svlog.Printf("client %d: wrote %v", cli.id, m)
			} else {
				err := fmt.Errorf("client %d: failed to write %v: %w", cli.id, m, err)
				svlog.Println(err)
				return
			}
		}
	}
}

func (cli client) consumeincoming(h hub) {
	defer cli.cancel()
	for {
		select {
		case <-cli.done():
			return
		case m := <-cli.inc:
			switch m.t {
			case mpong:
			case mjoin:
			case mexit:
			case msend:
			default:
				cli.trysend(cli.outc, errormes(errInvalidMessageType))
			}
		}
	}
}

func (cli client) resetping() {
	select {
	case <-cli.done():
	case cli.resetpingc <- zero{}:
	}
}

func (cli client) ping() {
	timer := time.NewTimer(pingInterval)
	defer func() {
		timer.Stop()
		svlog.Printf("client %d: ping stopped", cli.id)
	}()
	for {
		select {
		case <-cli.resetpingc:
		case <-cli.done():
			return
		case <-timer.C:
			if !cli.trysend(cli.outc, mes{t: mping}) {
				return
			}
		}
		timer.Reset(pingInterval)
	}
}

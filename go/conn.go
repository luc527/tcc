package main

import (
	"io"
	"iter"
	"log"
	"net"
	"time"
)

const (
	timeout = 60 * time.Second
)

type protoconn struct {
	ctx
	inc  chan protomes
	outc chan protomes
}

func (pc protoconn) trysend(c chan protomes, m protomes) bool {
	select {
	case c <- m:
		return true
	case <-pc.done():
		return false
	}
}

func (pc protoconn) produceinc(conn net.Conn) {
	defer pc.cancel()
	for {
		if pc.isdone() {
			return
		}

		deadline := time.Now().Add(timeout)
		if err := conn.SetReadDeadline(deadline); err != nil {
			log.Printf("failed to advance deadline: %v", err)
			return
		}

		m := protomes{}
		if _, err := m.ReadFrom(conn); err == nil {
			if !pc.trysend(pc.inc, m) {
				return
			}
		} else if perr, ok := err.(protoerror); ok {
			if !pc.trysend(pc.outc, errormes(perr)) {
				return
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

func (pc protoconn) consumeoutc(conn net.Conn) {
	defer pc.cancel()
	for {
		select {
		case <-pc.done():
			return
		case m := <-pc.outc:
			if _, err := m.WriteTo(conn); err != nil {
				return
			}
		}
	}
}

func (pc protoconn) messages() iter.Seq[protomes] {
	return func(yield func(protomes) bool) {
		defer pc.cancel()
		for {
			select {
			case m := <-pc.inc:
				if !yield(m) {
					return
				}
			case <-pc.done():
				return
			}
		}
	}
}

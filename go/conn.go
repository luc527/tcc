package main

import (
	"fmt"
	"io"
	"iter"
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

func makeconn(c ctx) protoconn {
	return protoconn{
		ctx:  c,
		inc:  make(chan protomes),
		outc: make(chan protomes),
	}
}

func (conn protoconn) trysend(dest chan protomes, m protomes) bool {
	select {
	case <-conn.done():
		return false
	case dest <- m:
		return true
	}
}

func (conn protoconn) readsingle(rawconn net.Conn) error {
	deadline := time.Now().Add(timeout)
	if err := rawconn.SetReadDeadline(deadline); err != nil {
		return err
	}

	m := protomes{}
	if _, err := m.ReadFrom(rawconn); err != nil {
		if merr, ok := err.(protoerror); ok {
			if !conn.trysend(conn.outc, errormes(merr)) {
				return fmt.Errorf("failed to send merror %w", err)
			}
		} else {
			return err
		}
	}

	if !conn.trysend(conn.inc, m) {
		return fmt.Errorf("failed to send message %v", m)
	}

	return nil
}

func (conn protoconn) writeoutgoing(w io.Writer) {
	defer conn.cancel()
	for {
		select {
		case <-conn.done():
			return
		case m := <-conn.outc:
			if _, err := m.WriteTo(w); err != nil {
				return
			}
		}
	}
}

func (conn protoconn) messages() iter.Seq[protomes] {
	return func(yield func(protomes) bool) {
		defer conn.cancel()
		for {
			select {
			case <-conn.done():
				return
			case m, ok := <-conn.inc:
				if !ok {
					return
				}
				if m.t == mping {
					m := protomes{t: mpong}
					if !conn.trysend(conn.outc, m) {
						return
					}
				}
				if !yield(m) {
					return
				}
			}
		}
	}
}

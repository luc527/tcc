package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"regexp"
	"time"
)

var (
	reMsgid    = regexp.MustCompile(`(\d+)-(\d+)`)
	errNomatch = fmt.Errorf("no match")
)

func prf(pre string, f string, a ...any) {
	t := time.Now().Unix()
	fmt.Printf("%s: %d %s\n", pre, t, fmt.Sprintf(f, a...))
}

func dbg(f string, a ...any) {
	prf("dbg", f, a...)
}

func pingConn(ctx context.Context, conn net.Conn) {
	tick := time.Tick(50 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick:
			m := msg{t: pingMsg}
			if _, err := m.WriteTo(conn); err != nil {
				dbg("failed to ping: %v", err)
				return
			}
		}
	}
}

// }

type testconn struct {
	c net.Conn
}

func (tc testconn) subscribe(topic uint16) bool {
	c := tc.c
	m := msg{t: subMsg, topic: topic}
	if _, err := m.WriteTo(c); err != nil {
		dbg("failed to subscribe: %v", err)
		return false
	}
	return true
}

func (tc testconn) publish(topic uint16, payload string) bool {
	c := tc.c
	m := msg{t: pubMsg, topic: topic, payload: payload}
	if _, err := m.WriteTo(c); err != nil {
		dbg("failed to publish: %v", err)
		return false
	}
	return true
}

func (tc testconn) waitPublication(topic uint16, payload string, timeout time.Duration) bool {
	c := tc.c

	d := make(chan zero)
	time.AfterFunc(timeout, func() { close(d) })

	m := msg{}
	for {
		select {
		case <-d:
			return false
		default:
		}
		if _, err := m.ReadFrom(c); err != nil {
			if err != io.EOF {
				dbg("failed to read, waiting for publication: %v", err)
			}
			return false
		}
		if m.t == pubMsg && m.topic == topic && m.payload == payload {
			select {
			case <-d:
				return false
			default:
				return true
			}
		}
	}
}

// go impl is actually the one with worst throughput
// strace shows lots of futex, so... channels
// (btw, the node strace is the cleanest one! just a lot of write() calls)

func testThroughput(address string) {
	// TODO: did a git checkout . on the whole go folder :-)
	// so... reimplement this

	// 20 topics
	// 5 publishers
	// 10, 100, 1000, 2000, 3000 subscribers
}

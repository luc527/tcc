package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"regexp"
	"strings"
	"sync"
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
	tick := time.Tick(30 * time.Second)
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

const (
	bigpayload = true
)

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
		if m.t == pubMsg && m.topic == topic && strings.Index(m.payload, payload) == 0 {
			select {
			case <-d:
				return false
			default:
				return true
			}
		}
	}
}

func throughputPublisher(ctx context.Context, cancel context.CancelFunc, wg *sync.WaitGroup, conn net.Conn, topic uint16, id int) {
	defer func() {
		dbg("topic=%d pub=%d terminating", topic, id)
		cancel()
		conn.Close()
		wg.Done()
	}()

	tconn := testconn{conn}

	if !tconn.subscribe(topic) {
		dbg("publisher failed to subscribe")
		return
	}

	done := func() bool {
		select {
		case <-ctx.Done():
			return true
		default:
			return false
		}
	}

	var bb *bytes.Buffer
	var bs []byte
	if bigpayload {
		bb = new(bytes.Buffer)
		bs = make([]byte, 2048)
		if _, err := rand.Read(bs); err != nil {
			dbg("topic=%d pub=%d failed to generate random bytes for big payload", topic, id)
		}
		for i := range bs {
			bs[i] = 'a' + bs[i]%26
		}
	}

	tframe := newThroughputFrame(10 * time.Second)

	msgi := 0
	for {
		if done() {
			return
		}

		pl0 := fmt.Sprintf("pub %d msg %d", id, msgi)
		pl1 := pl0
		if bigpayload {
			bb.Reset()
			bb.WriteString(pl0)
			bb.Write(bs)
			bb.WriteString(pl0)
			pl1 = bb.String()
		}
		if !tconn.publish(topic, pl1) {
			return
		}
		sent := time.Now()
		measurement := tframe.onsend()
		msgi++

		if !tconn.waitPublication(topic, pl0, 1*time.Minute) {
			dbg("publisher failed to wait for publication")
			return
		}
		if done() {
			return
		}
		delay := time.Since(sent)

		delayMs := delay.Milliseconds()
		prf("pub", "topic=%d pub=%d msg=%d delayMs=%d frame=%d throughputPsec=%.4f", topic, id, msgi, delayMs, measurement.i, measurement.v)
	}
}

func throughputSubscriber(ctx context.Context, cancel context.CancelFunc, wg *sync.WaitGroup, conn net.Conn, topic uint16) {
	defer func() {
		cancel()
		conn.Close()
		wg.Done()
	}()

	go io.Copy(io.Discard, conn)

	tconn := testconn{conn}
	if !tconn.subscribe(topic) {
		dbg("subscriber failed to subscribe")
		return
	}

	<-ctx.Done()
}

func testThroughput(address string) {
	ctx0, cancel0 := context.WithCancel(context.Background())
	wg0 := new(sync.WaitGroup)

	primes := []int{2, 3, 5, 7, 11, 13, 17, 19, 23, 29, 31, 37, 41, 43, 47, 53, 59, 61, 67, 71, 73, 79, 83, 89, 97, 101, 103, 107, 109, 113, 127, 131}

	const (
		ntopic      = 28
		npubs       = 5
		nconn       = 800
		subsPerIter = 4
	)

	npubconns := npubs * ntopic
	dbg("creating %d publisher connections", npubconns)
	pubconns := multiconnect(nil, npubconns, 25, address)
	dbg("created %d publisher connections", npubconns)
	for ti := range ntopic {
		for pubi := range npubs {
			conn := pubconns[int(ti)*npubs+pubi]
			ctx, cancel := context.WithCancel(ctx0)

			wg0.Add(1)
			topic := uint16(ti + primes[ti])
			go throughputPublisher(ctx, cancel, wg0, conn, topic, pubi)
		}
		time.Sleep(100 * time.Millisecond)
	}

	dbg("creating %d subscriber connections", nconn)
	conns := multiconnect(nil, nconn, 32, address)
	dbg("created %d subscriber connections", nconn)
	for i, conn := range conns {
		ctx, cancel := context.WithCancel(ctx0)
		wg0.Add(1)
		go io.Copy(io.Discard, conn)
		go func() {
			defer func() {
				dbg("conn %d terminating", i)
				conn.Close()
				cancel()
				wg0.Done()
			}()
			pingConn(ctx, conn)
		}()
	}

	for it := 0; it < ntopic/subsPerIter; it++ {
		dbg("iteration %d, topics per conn %d", it, subsPerIter*(it+1))
		for i, conn := range conns {
			tconn := testconn{conn}
			base := subsPerIter * (it + i) % ntopic
			for j := range subsPerIter {
				ti := base + j
				topic := uint16(ti + primes[ti])
				tconn.subscribe(topic)
			}
			dbg("conn %d subscribed to %d through %d", i, base, base+subsPerIter-1)
		}
		dbg("iteration %d finished subscribing", it)

		time.Sleep(30 * time.Second)
	}

	dbg("finishing")
	cancel0()
	wg0.Wait()
	dbg("finished")
}

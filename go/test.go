package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"regexp"
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

func throughputPublisher(ctx context.Context, cancel context.CancelFunc, wg *sync.WaitGroup, conn net.Conn, topic uint16, id int) {
	defer func() {
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

	tframe := newThroughputFrame(10 * time.Second)

	msgi := 0
	for {
		if done() {
			return
		}

		pl := fmt.Sprintf("pub %d msg %d", id, msgi)
		if !tconn.publish(topic, pl) {
			return
		}
		sent := time.Now()
		measurement := tframe.onsend()
		msgi++

		if !tconn.waitPublication(topic, pl, 1*time.Minute) {
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

	const (
		ntopic = 20
		npubs  = 5
		nconn  = 800
	)

	npubconns := npubs * ntopic
	dbg("creating %d publisher connections", npubconns)
	pubconns := multiconnect(nil, npubconns, 25, address)
	for topic := range uint16(ntopic) {
		for pubi := range npubs {
			conn := pubconns[int(topic)*npubs+pubi]
			ctx, cancel := context.WithCancel(ctx0)

			wg0.Add(1)
			go throughputPublisher(ctx, cancel, wg0, conn, topic, pubi)
		}
	}

	dbg("creating %d subscriber connections", nconn)
	subconni := 0
	subconns := multiconnect(nil, nconn, 20, address)
	subctxs := make([]context.Context, nconn)
	subcancels := make([]context.CancelFunc, nconn)
	for i, conn := range subconns {
		ctx, cancel := context.WithCancel(ctx0)
		subctxs[i] = ctx
		subcancels[i] = cancel

		go pingConn(ctx, conn)
	}

	prevNsubs := 0
	incNsubs := []int{10, 100, 200, 400, 800, 1600, 3200, 6400}
	for _, nsubs := range incNsubs {
		dbg("subs: %d", nsubs)
		newNsubs := nsubs - prevNsubs

		tick := time.Tick(25 * time.Millisecond)
		for topic := range uint16(ntopic) {
			dbg("adding %d subs to %d", newNsubs, topic)
			for range newNsubs {
				conn, ctx, cancel := subconns[subconni], subctxs[subconni], subcancels[subconni]
				wg0.Add(1)
				go throughputSubscriber(ctx, cancel, wg0, conn, topic)
				subconni = (subconni + 1) % len(subconns)
			}
			<-tick
		}
		time.Sleep(30 * time.Second)
		prevNsubs = nsubs
	}

	dbg("finishing")
	cancel0()
	wg0.Wait()
	dbg("finished")
}

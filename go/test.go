package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"os"
	"slices"
	"strings"
	"sync"
	"time"
)

func fprf(w io.Writer, pre string, f string, a ...any) {
	t := time.Now().UnixMicro()
	fmt.Fprintf(w, "%s: %d %s\n", pre, t, fmt.Sprintf(f, a...))
}

func prf(pre string, f string, a ...any) {
	fprf(os.Stdout, pre, f, a...)
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
	bigpayload = false
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
			return
		}
		for i := range bs {
			bs[i] = 'a' + bs[i]%26
		}
	}

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
		prf("pub", "send topic=%d pub=%d", topic, id)
		sent := time.Now()
		msgi++

		if !tconn.waitPublication(topic, pl0, 1*time.Minute) {
			dbg("publisher failed to wait for publication")
			return
		}
		if done() {
			return
		}
		delay := time.Since(sent)
		prf("pub", "recv topic=%d pub=%d delayMs=%d", topic, id, delay.Milliseconds())
	}
}

func testThroughput(address string) {
	ctx0, cancel0 := context.WithCancel(context.Background())
	wg0 := new(sync.WaitGroup)

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
	for topic := range uint16(ntopic) {
		for pubi := range npubs {
			conn := pubconns[int(topic)*npubs+pubi]
			ctx, cancel := context.WithCancel(ctx0)

			wg0.Add(1)
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
				topic := uint16(base + j)
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

func latencyPublisher(ctx context.Context, wg *sync.WaitGroup, conn net.Conn, topic uint16, publisherIdx int, pubInterval time.Duration) {
	go io.Copy(io.Discard, conn) // just a guarantee; unnecessary since publisher won't subscribe
	defer func() {
		dbg("publisher %3d of topic %3d terminating", publisherIdx, topic)
		conn.Close()
		wg.Done()
	}()
	dbg("publisher %3d of topic %3d started", publisherIdx, topic)

	tick := time.Tick(pubInterval)
	publicationIdx := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick:
			tconn := testconn{conn}
			payload := fmt.Sprintf("pubsher %d, pubton %d", publisherIdx, publicationIdx)
			if !tconn.publish(topic, payload) {
				return
			}
			prf("pub", "topic=%d payload=%s", topic, payload)
		}
		publicationIdx++
	}
}

func latencySubscriber(ctx context.Context, wg *sync.WaitGroup, conn net.Conn) {
	defer func() {
		conn.Close()
		wg.Done()
	}()

	go pingConn(ctx, conn)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		m := msg{}
		if _, err := m.ReadFrom(conn); err != nil {
			if err != io.EOF {
				dbg("subscriber failed to read: %v", err)
			}
			return
		}

		if m.t == pubMsg {
			prf("sub", "topic=%d payload=%s", m.topic, m.payload)
		}
	}
}

func testLatency(address string) {
	const (
		numTopics             = 32
		numPublishersPerTopic = 4
		pubInterval           = 4 * time.Second
	)

	ctx0, cancel0 := context.WithCancel(context.Background())
	wg0 := new(sync.WaitGroup)

	numTotalPubConns := numTopics * numPublishersPerTopic
	dbg("starting %d publisher connections", numTotalPubConns)

	pubConns := multiconnect(nil, numTotalPubConns, 30, address)
	for publisherIdx := range numPublishersPerTopic {
		for topic := range uint16(numTopics) {
			connIdx := int(topic)*numPublishersPerTopic + publisherIdx
			conn := pubConns[connIdx]
			if conn == nil {
				dbg("skipping nil publisher")
				continue
			}
			wg0.Add(1)
			go latencyPublisher(ctx0, wg0, conn, topic, publisherIdx, pubInterval)
			time.Sleep(137 * time.Millisecond)
		}
	}

	// 1st part: each subscriber will be a new connection
	// so in total incNumConnSubs[-1] * numTopics connections
	incNumConnSubs := []int{60, 120}

	topicsPerConn := make(map[net.Conn][]uint16)

	prevNumSubs := 0
	for _, numSubs := range incNumConnSubs {
		numNewSubs := numSubs - prevNumSubs
		numNewConns := numNewSubs * numTopics
		dbg("%d subs per topic, %d new connections", numSubs, numNewConns)

		conns := multiconnect(nil, numNewConns, 30, address)
		for _, conn := range conns {
			if conn == nil {
				dbg("skipping nil subscriber (0)")
				continue
			}
			wg0.Add(1)
			go latencySubscriber(ctx0, wg0, conn)
		}

		connsToSubscribe := conns
		for topic := range uint16(numTopics) {
			for range numNewSubs {
				var conn net.Conn
				conn, connsToSubscribe = connsToSubscribe[0], connsToSubscribe[1:]
				if conn == nil {
					dbg("skipping nil subscriber (1)")
					continue
				}
				tconn := testconn{conn}
				go tconn.subscribe(topic)
				topicsPerConn[conn] = append(topicsPerConn[conn], topic)
			}
		}

		prevNumSubs = numSubs
		time.Sleep(30 * time.Second)
	}

	// now we'll reuse connections for subscribers
	incNumSubs := []int{240, 480, 960, 1920}

	prevNumSubs = incNumConnSubs[len(incNumConnSubs)-1]
	for _, numSubs := range incNumSubs {
		numNewSubs := numSubs - prevNumSubs
		dbg("%d subs per topic, reusing connections", numSubs)

		for topic := range uint16(numTopics) {
			wg := new(sync.WaitGroup)
			for range numNewSubs {
				// find a connection that hasn't subscribed to this topic yet
				var conn net.Conn
				found := false
				for conn_, topics := range topicsPerConn {
					if !slices.Contains(topics, topic) {
						found = true
						conn = conn_
						break
					}
				}
				if !found {
					panic("no connections available")
				}
				if conn == nil {
					dbg("skipping nil subscriber (2)")
					continue
				}
				wg.Add(1)
				go func() {
					defer wg.Done()
					tconn := testconn{conn}
					tconn.subscribe(topic)
				}()
				topicsPerConn[conn] = append(topicsPerConn[conn], topic)
			}
			wg.Wait()
			time.Sleep(199 * time.Millisecond)
		}

		time.Sleep(30 * time.Second)
		prevNumSubs = numSubs
	}

	dbg("finishing latency test")
	cancel0()
	wg0.Done()
	dbg("finished latency test")
}

// next tests:
// messages involving computation (what to measure?)
// connections and subscribers coming ang going
// ...

// obs: ignore throughputbig

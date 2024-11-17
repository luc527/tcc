package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"regexp"
	"strconv"
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

func dbg2(f string, a ...any) {
	// dbg(f, a...)
	_ = f
	_ = a
}

func prependMsgid(payload string, connid int, msgid int) string {
	return fmt.Sprintf("%d-%d %s", connid, msgid, payload)
}

func extractMsgid(payload string, wantConnid int) (int, error) {
	ms := reMsgid.FindStringSubmatch(payload)
	if len(ms) != 3 {
		return 0, fmt.Errorf("extract msg: regexp failed")
	}
	gotConnid_, err := strconv.ParseInt(ms[1], 10, 32)
	if err != nil {
		return 0, fmt.Errorf("extract msg connid: %w", err)
	}
	gotConnid := int(gotConnid_)
	if gotConnid != wantConnid {
		return 0, errNomatch
	}
	msgid_, err := strconv.ParseInt(ms[2], 10, 32)
	if err != nil {
		return 0, fmt.Errorf("extract msg id: %w", err)
	}
	msgid := int(msgid_)
	return msgid, nil
}

// func (tc *testconn) runMeasurer() {
// 	pending := make(map[int]time.Time)
// 	for {
// 		select {
// 		case <-tc.ctx.Done():
// 			return
// 		case x := <-tc.sends:
// 			pending[x.i] = x.t
// 		case x := <-tc.recvs:
// 			if sendt, ok := pending[x.i]; ok {
// 				recvt := x.t
// 				observation(x.topic, sendt, recvt)
// 			} else {
// 				dbg("recv without send?! connid: %d, msgid: %d", tc.id, x.i)
// 			}
// 		}
// 	}
// }

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

type connector struct {
	p       int
	address string
	done    chan zero
	work    chan zero
	conn    chan net.Conn
}

func makeConnector(p int, address string) connector {
	return connector{
		p:       p,
		address: address,
		done:    make(chan zero),
		work:    make(chan zero, p),
		conn:    make(chan net.Conn),
	}
}

func (c *connector) start() {
	for range c.p {
		go func() {
			for range c.work {
				conn, err := net.Dial("tcp", c.address)
				if err != nil {
					dbg("failed to connect: %v", err)
				}
				select {
				case <-c.done:
				case c.conn <- conn:
				}
			}
		}()
	}
}

func (c *connector) stop() {
	close(c.done)
	close(c.work)
}

type connpool struct {
	ctor connector
	n    int
	i    int
	a    []net.Conn
}

func newConnpool(n int, ctor connector) *connpool {
	return &connpool{
		ctor: ctor,
		n:    n,
		a:    nil,
	}
}

// TODO: redo
// func (cp *connpool) getConn() net.Conn {
// 	if len(cp.a) < cp.n {
// 		conn := cp.ctor.connect()
// 		if conn == nil {
// 			return nil
// 		}
// 		cp.a = append(cp.a)
// 		return conn
// 	} else {
// 		j := cp.i
// 		cp.i++
// 		return cp.a[j]
// 	}
// }

type testdriver struct {
	ctx    context.Context
	cancel context.CancelFunc
	ctor   connector
	wg     sync.WaitGroup
}

func newTestdriver(parent context.Context, address string) *testdriver {
	ctx, cancel := context.WithCancel(parent)

	ctor := makeConnector(32, address)
	ctor.start()
	context.AfterFunc(ctx, ctor.stop)

	tg := &testdriver{
		ctx:    ctx,
		cancel: cancel,
		ctor:   ctor,
		wg:     sync.WaitGroup{},
	}
	return tg
}

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

func throughputPublisher(
	ctx context.Context,
	cancel context.CancelFunc,
	conn net.Conn,
	wg *sync.WaitGroup,
	topic uint16,
	id int,
) {
	defer func() {
		conn.Close()
		cancel()
		wg.Done()
	}()

	done := func() bool {
		select {
		case <-ctx.Done():
			return true
		default:
			return false
		}
	}

	tc := testconn{conn}

	if !tc.subscribe(topic) {
		return
	}

	start := time.Now()
	sent := 0

	for {
		if done() {
			return
		}

		pl := fmt.Sprintf("%d-%d", id, sent)
		if !tc.publish(topic, pl) || done() {
			return
		}
		sentt := time.Now()
		sent++

		timeout := 1 * time.Minute
		if !tc.waitPublication(topic, pl, timeout) {
			prf("pub", "timed out waiting (timeout %ds)", int(timeout.Seconds()))
			return
		}
		if done() {
			return
		}

		elapsed := sentt.Sub(start)
		delay := time.Since(sentt)

		elapsedSec := elapsed.Seconds()
		throughput := float64(0)
		if elapsedSec > 0 {
			throughput = float64(sent) / elapsed.Seconds()
		}

		prf(
			"pub",
			"throughput topic=%d pubid=%d sent=%d elapsed_ms=%d throughput_persec=%.4f delay_ms=%d",
			topic,
			id,
			sent,
			elapsed.Milliseconds(),
			throughput,
			delay.Milliseconds(),
		)
	}
}

func testThroughput(address string) {
	ctx0, cancel0 := context.WithCancel(context.Background())
	defer cancel0()

	wg0 := new(sync.WaitGroup)

	ctor := makeConnector(50, address)
	ctor.start()
	context.AfterFunc(ctx0, ctor.stop)

	const (
		ntopic = uint16(50)
		npub   = 10
	)

	go func() {
		for range npub * ntopic {
			ctor.work <- zero{}
		}
	}()

	for topic := range ntopic {
		for id := range npub {
			c := <-ctor.conn
			if c == nil {
				dbg("failed to connect publisher")
				continue
			}
			dbg("connected publisher %d of topic %d", id, topic)
			ctx, cancel := context.WithCancel(ctx0)
			wg0.Add(1)
			go throughputPublisher(ctx, cancel, c, wg0, topic, id)
		}
		dbg("connected topic %d", topic)
	}

	dbg("finished connecting")
	time.Sleep(1 * time.Minute)

	// throughputnosub has this segment commented {
	// cpool := newConnpool(4000, ctor)

	// prevNsubs := 0
	// incNsubs := []int{10, 100, 1000, 2000, 4000, 8000, 16000, 32000}

	// for _, nsubs := range incNsubs {
	// 	nNewsubs := nsubs - prevNsubs

	// 	prevNsubs = nsubs
	// }
	// }

	dbg("finishing throughput test")
	cancel0()
	wg0.Wait()
}

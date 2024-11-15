package main

import (
	"context"
	"fmt"
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

func dbg(f string, a ...any) {
	fmt.Printf("dbg: %d %s\n", time.Now().Unix(), fmt.Sprintf(f, a...))
}

func dbg2(f string, a ...any) {
	// dbg(f, a...)
	_ = f
	_ = a
}

func observation(topic uint16, sendt time.Time, recvt time.Time) {
	delayMs := recvt.Sub(sendt).Milliseconds()
	fmt.Printf("observation: %d topic=%d delayMs=%d\n", sendt.Unix(), topic, delayMs)
}

type itime struct {
	i     int
	t     time.Time
	topic uint16
}

type testconn struct {
	id     int
	ctx    context.Context
	cancel context.CancelFunc
	conn   net.Conn
	subc   chan subscription
	pubc   chan publication
	sends  chan itime
	recvs  chan itime
}

func newTestconn(id int, parent context.Context, conn net.Conn) *testconn {
	ctx, cancel := context.WithCancel(parent)
	return &testconn{
		id:     id,
		ctx:    ctx,
		conn:   conn,
		cancel: cancel,
		subc:   make(chan subscription),
		pubc:   make(chan publication),
		sends:  make(chan itime),
		recvs:  make(chan itime),
	}
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

func (tc *testconn) runWriter() {
	msgi := 0
	for {
		select {
		case <-tc.ctx.Done():
			return
		case sx := <-tc.subc:
			m := msg{t: subMsg, topic: sx.topic, b: sx.subscribed}
			if _, err := m.WriteTo(tc.conn); err != nil {
				dbg("failed to subscribe: %v", err)
				return
			}
			dbg2("testconn %d wrote: %v", tc.id, m)
		case px := <-tc.pubc:
			payload := prependMsgid(px.payload, tc.id, msgi)
			m := msg{t: pubMsg, topic: px.topic, payload: payload}
			if _, err := m.WriteTo(tc.conn); err != nil {
				dbg("failed to publish: %v", err)
				return
			}
			dbg2("testconn %d wrote: %v", tc.id, m)
			select {
			case <-tc.ctx.Done():
			case tc.sends <- itime{i: msgi, t: time.Now(), topic: px.topic}:
			}
			msgi++
		}
	}
}

func (tc *testconn) runMeasurer() {
	pending := make(map[int]time.Time)
	for {
		select {
		case <-tc.ctx.Done():
			return
		case x := <-tc.sends:
			pending[x.i] = x.t
		case x := <-tc.recvs:
			if sendt, ok := pending[x.i]; ok {
				recvt := x.t
				observation(x.topic, sendt, recvt)
			} else {
				dbg("recv without send?! connid: %d, msgid: %d", tc.id, x.i)
			}
		}
	}
}

func (tc *testconn) runReader() {
	for {
		m := msg{}
		if _, err := m.ReadFrom(tc.conn); err != nil {
			dbg("failed to read: %v", err)
			return
		}
		dbg2("testconn %d read: %v", tc.id, m)
		if m.t == pubMsg {
			msgid, err := extractMsgid(m.payload, tc.id)
			if err != nil {
				if err != errNomatch {
					dbg("failed to extract msgid: %v", err)
				}
				continue
			}
			select {
			case <-tc.ctx.Done():
			case tc.recvs <- itime{i: msgid, t: time.Now(), topic: m.topic}:
			}
		}
	}
}

func (tc *testconn) run(wg *sync.WaitGroup) {
	defer tc.cancel()
	defer wg.Done()

	dbg("testconn %d started", tc.id)
	defer dbg("testconn %d closed", tc.id)

	go pingConn(tc.ctx, tc.conn)
	go tc.runMeasurer()
	go tc.runReader()

	tc.runWriter()
}

func (tc *testconn) subscribe(topic uint16) {
	select {
	case <-tc.ctx.Done():
	case tc.subc <- subscription{topic, true}:
	}
}

func (tc *testconn) publish(topic uint16, payload string) {
	select {
	case <-tc.ctx.Done():
	case tc.pubc <- publication{topic: topic, payload: payload}:
	}
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

func multiConnect(n int, address string, p int) <-chan net.Conn {
	work := make(chan zero)
	go func() {
		for range n {
			work <- zero{}
		}
		close(work)
	}()

	wg := new(sync.WaitGroup)
	ret := make(chan net.Conn)
	for range p {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range work {
				conn, err := net.Dial("tcp", address)
				if err != nil {
					dbg("failed to connect: %v", err)
				}
				ret <- conn
			}
		}()
	}

	go func() {
		wg.Wait()
		close(ret)
	}()

	return ret
}

type testconnPool struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	n      int
	i      int
	a      []*testconn
}

func newTestconnPool(n int, parent context.Context) *testconnPool {
	ctx, cancel := context.WithCancel(parent)
	tp := &testconnPool{
		ctx:    ctx,
		cancel: cancel,
		wg:     sync.WaitGroup{},
		n:      n,
		i:      0,
		a:      make([]*testconn, n),
	}
	return tp
}

func (tp *testconnPool) start(address string) {
	// TODO: instead of pre-creating all connections
	// create connections only when asked, but stop creating and start reusing previous connections
	// when n is reached

	// TODO: something about the observations' delayMs is fishy
	// increases almost monotonically with every burst of publications -- on localhost, at least
	// maybe that's expected?

	// -- also happens with the node impl, so not a go impl specific thing

	// TODO: maybe should be able to activate measurer for a single topic only?
	// would that solve the other problem? cascading delay

	const p = 32
	i := 0
	for conn := range multiConnect(tp.n, address, p) {
		if conn == nil {
			continue
		}
		tc := newTestconn(i, tp.ctx, conn)
		tp.a[i] = tc
		i++

		tp.wg.Add(1)
		go tc.run(&tp.wg)
	}
	dbg("test conn pool started")
}

func (tp *testconnPool) choose() *testconn {
	i := tp.i
	tp.i = (i + 1) % len(tp.a)
	return tp.a[i]
}

func (tp *testconnPool) stop() {
	tp.cancel()
	tp.wg.Wait()
}

func runPublisher(tc *testconn, topic uint16, interval time.Duration, msgf func() string) {
	tick := time.Tick(interval)
	done := tc.ctx.Done()
	for {
		select {
		case <-done:
			return
		case <-tick:
			tc.publish(topic, msgf())
		}
	}
}

func testTest(address string) {
	const nconn = 10
	tp := newTestconnPool(nconn, context.Background())
	tp.start(address)

	topics := []uint16{1, 10, 100}
	for _, topic := range topics {
		for range 3 {
			tc := tp.choose()
			tc.subscribe(topic)
			go runPublisher(tc, topic, 7*time.Second, func() string { return "hello world" })
		}
		for range 5 {
			tp.choose().subscribe(topic)
		}

		time.Sleep(30 * time.Second)
	}

	tp.stop()
}

func test0(address string) {
	// just for collecting some delay data from different implementations
	const nconn = 100
	const topic = 123

	tp := newTestconnPool(nconn, context.Background())
	tp.start(address)

	for range nconn {
		tc := tp.choose()
		tc.subscribe(topic)
		go runPublisher(tc, topic, 2*time.Second, func() string { return "hello" })
		time.Sleep(100 * time.Millisecond)
	}

	time.Sleep(30 * time.Second)
	tp.stop()
}

func testIncreasingTopics(address string) {
	// TODO: change connection pool size, see if performance changes much
	const nconn = 4096
	tp := newTestconnPool(nconn, context.Background())
	tp.start(address)

	topic := uint16(0)
	nextTopic := func() uint16 {
		t := topic
		topic++
		return t
	}

	prevTopcount := 0
	topcounts := []int{10, 80, 320, 640, 1280, 2560, 5120}
	for _, topcount := range topcounts {
		newtopCount := topcount - prevTopcount
		for range newtopCount {
			topic := nextTopic()
			for range 30 {
				tp.choose().subscribe(topic)
			}
			for range 10 {
				tc := tp.choose()
				tc.subscribe(topic)
				go runPublisher(tc, topic, 10*time.Second, func() string { return "hello world" })
			}
		}
		prevTopcount = topcount

		time.Sleep(61 * time.Second)
	}

	tp.stop()
}

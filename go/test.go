package main

// TODO: connect to server gradually
// use fixed number of goroutines to actually call net.Dial, round-robin

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// TODO: maybe print dbg err msg only if channel not done yet

func runSubscriber(done <-chan zero, conn net.Conn, topic uint16) {
	go io.Copy(io.Discard, conn)

	m := msg{t: subMsg, topic: topic, b: true}
	if _, err := m.WriteTo(conn); err != nil {
		dbg("unable to subscribe: %v", err)
		return
	}

	tick := time.Tick(46 * time.Second)
	for {
		select {
		case <-done:
			return
		case <-tick:
			m := msg{t: pingMsg}
			if _, err := m.WriteTo(conn); err != nil {
				dbg("unable to ping: %v", err)
				return
			}
		}
	}
}

func runPublisher(done <-chan zero, conn net.Conn, topic uint16, interval time.Duration, gen func() string) {
	go io.Copy(io.Discard, conn)

	tick := time.Tick(interval)
	for range tick {
		select {
		case <-done:
			return
		case <-tick:
			p := gen()
			m := msg{t: pubMsg, topic: topic, payload: p}
			if _, err := m.WriteTo(conn); err != nil {
				dbg("unable to publish: %v", err)
				return
			}
		}
	}
}

func runMonitor(id int, done <-chan zero, conn net.Conn, topic uint16, interval time.Duration) {
	m := msg{t: subMsg, topic: topic, b: true}
	if _, err := m.WriteTo(conn); err != nil {
		dbg("unable to subscribe: %v", err)
		return
	}

	type itime struct {
		i int
		t time.Time
	}

	sends := make(chan itime)
	recvs := make(chan itime)

	prefix := fmt.Sprintf("mon(%d)", id)
	go func() {
		tick := time.Tick(interval)

		i := 0
		for {
			select {
			case <-done:
				return
			case <-tick:
				p := fmt.Sprintf("%v%v", prefix, i)
				m := msg{t: pubMsg, topic: topic, payload: p}
				if _, err := m.WriteTo(conn); err != nil {
					dbg("unable to publish: %v", err)
					return
				}
				select {
				case <-done:
				case sends <- itime{i, time.Now()}:
				}
				i++
			}
		}
	}()

	go func() {
		for {
			m := msg{}
			if _, err := m.ReadFrom(conn); err != nil {
				if err != io.EOF {
					dbg("unable to read: %v", err)
				}
				return
			}
			t := time.Now()
			if m.t == pubMsg && m.topic == topic && strings.Index(m.payload, prefix) == 0 {
				i__ := m.payload[len(prefix):]
				i_, err := strconv.ParseInt(i__, 10, 32)
				if err != nil {
					dbg("unable to recover monitor message id: %v", err)
					return
				}
				i := int(i_)
				select {
				case <-done:
				case recvs <- itime{i, t}:
				}
			}
		}
	}()

	sendtimes := make(map[int]time.Time)

	for {
		select {
		case <-done:
			return
		case x := <-sends:
			sendtimes[x.i] = x.t
		case x := <-recvs:
			if sendtime, ok := sendtimes[x.i]; ok {
				delay := x.t.Sub(sendtime)
				out("observation topic=%v delayMs=%v", topic, delay.Milliseconds())
			} else {
				dbg("recv without send?!")
			}
		}
	}
}

func dbg(f string, args ...any) {
	fmt.Printf("dbg: %d %v\n", time.Now().Unix(), fmt.Sprintf(f, args...))
}

// TODO: in/out didn't really help, could've just been 'observation', 'subs' etc directly as prefix

func in(f string, args ...any) {
	fmt.Printf("in: %d %v\n", time.Now().Unix(), fmt.Sprintf(f, args...))
}

func out(f string, args ...any) {
	fmt.Printf("out: %d %v\n", time.Now().Unix(), fmt.Sprintf(f, args...))
}

type taskGroup struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func newTaskGroup(ctx context.Context) *taskGroup {
	ctx, cancel := context.WithCancel(ctx)
	tg := &taskGroup{
		ctx:    ctx,
		cancel: cancel,
		wg:     sync.WaitGroup{},
	}
	tg.wg.Add(1)
	return tg
}

func (tg *taskGroup) do(name string, f0 func(context.Context)) {
	tg.wg.Add(1)
	ctx, cancel := context.WithCancel(tg.ctx)
	f1 := func() {
		defer tg.wg.Done()
		defer cancel()
		defer dbg("task %q ended", name)
		f0(ctx)
	}
	dbg("task %q started", name)
	go f1()
}

func (tg *taskGroup) stop() {
	tg.wg.Done()
	tg.cancel()
	tg.wg.Wait()
}

type testDriver struct {
	tg        *taskGroup
	address   string
	nextPubid int
	nextMonid int
}

func (td *testDriver) pubid() int {
	td.nextPubid++
	return td.nextPubid
}

func (td *testDriver) monid() int {
	td.nextMonid++
	return td.nextMonid
}

func newTestDriver(ctx context.Context, address string) *testDriver {
	td := &testDriver{
		tg:      newTaskGroup(ctx),
		address: address,
	}

	return td
}

func (td *testDriver) spawnMonitors(n int, topic uint16, interval time.Duration) {
	in("monitors n=%v topic=%v intervalMs=%v", n, topic, interval.Milliseconds())
	for range n {
		id := td.monid()
		name := fmt.Sprintf("m%d", id)
		td.tg.do(name, func(ctx context.Context) {
			conn, err := net.Dial("tcp", td.address)
			if err != nil {
				dbg("%v failed to connect: %v", name, err)
				return
			}
			defer conn.Close()

			runMonitor(id, ctx.Done(), conn, topic, interval)
		})
	}
}

func (td *testDriver) spawnSubs(n int, topic uint16) {
	in("subs n=%d topic=%d", n, topic)
	for range n {
		td.tg.do("subscriber", func(ctx context.Context) {
			conn, err := net.Dial("tcp", td.address)
			if err != nil {
				dbg("subscriber failed to connect: %v", err)
				return
			}
			defer conn.Close()

			runSubscriber(ctx.Done(), conn, topic)
		})
	}
}

type payloadGenerator struct {
	name string
	f    func() string
}

func (td *testDriver) spawnPubs(n int, topic uint16, interval time.Duration, gen payloadGenerator) {
	in("pubs n=%v topic=%v intervalMs=%v payload=%v", n, topic, interval.Milliseconds(), gen.name)

	for range n {
		name := fmt.Sprintf("p%d", td.pubid())
		td.tg.do(name, func(ctx context.Context) {
			conn, err := net.Dial("tcp", td.address)
			if err != nil {
				dbg("%v failed to connect: %v", name, err)
				return
			}
			defer conn.Close()

			runPublisher(ctx.Done(), conn, topic, interval, gen.f)
		})
	}
}

// @tests

func test0(ctx context.Context, address string) {
	td := newTestDriver(ctx, address)

	td.spawnMonitors(10, 15, 5*time.Second)
	time.Sleep(30 * time.Second)
	td.spawnMonitors(100, 15, 5*time.Second)
	time.Sleep(30 * time.Second)
	td.spawnMonitors(100, 15, 5*time.Second)
	time.Sleep(30 * time.Second)

	td.tg.stop()
}

var (
	fixedPayload = payloadGenerator{
		name: "fixed",
		f: func() string {
			return "hello world"
		},
	}
)

func testIncreasingSubs(ctx context.Context, address string) {
	td := newTestDriver(ctx, address)

	const (
		topic = uint16(765)
	)

	prevSubcount := 0
	subcounts := []int{10, 80, 640, 5120}

	for range 10 {
		td.spawnPubs(1, topic, 7*time.Second, fixedPayload)
		td.spawnMonitors(2, topic, 10*time.Second)
		time.Sleep(1 * time.Second)
	}

	for _, subcount := range subcounts {
		newSubcount := subcount - prevSubcount

		td.spawnSubs(newSubcount, topic)

		time.Sleep(1 * time.Minute)

		prevSubcount = subcount
	}

	td.tg.stop()
}

package main

import (
	"context"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// @taskGroup

type task struct {
	name string
	f    func()
}

type taskGroup struct {
	d      chan zero
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func newTaskGroup(ctx context.Context) *taskGroup {
	ctx, cancel := context.WithCancel(ctx)
	tg := &taskGroup{
		d:      make(chan zero),
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
		defer fmt.Printf("dbg: task %q ended\n", name)
		f0(ctx)
	}
	fmt.Printf("dbg: task %q started\n", name)
	go f1()
}

func (tg *taskGroup) stop() {
	tg.wg.Done()
	tg.cancel()
}

func (tg *taskGroup) wait() {
	tg.wg.Wait()
	close(tg.d)
}

func (tg *taskGroup) done() <-chan zero {
	return tg.d
}

// @testDriver

type idtime struct {
	i int
	t time.Time
}

type observation struct {
	send time.Time
	recv time.Time
}

type monitorObservation struct {
	topic uint16
	send  time.Time
	recv  time.Time
}

type testDriver struct {
	tg      *taskGroup
	address string
	mobs    []monitorObservation
	mobc    chan monitorObservation
}

func newTestDriver(ctx context.Context, address string) *testDriver {
	td := &testDriver{
		tg:      newTaskGroup(ctx),
		address: address,
		mobc:    make(chan monitorObservation),
	}

	go func() {
		for {
			select {
			case <-td.tg.done():
				fmt.Printf("dbg: test driver mobs goroutine ended\n")
				return
			case mob := <-td.mobc:
				fmt.Printf("dbg: mob %v, %v\n", mob.topic, mob.recv.Sub(mob.send))
				td.mobs = append(td.mobs, mob)
			}
		}
	}()

	return td
}

func (td *testDriver) spawnMonitors(n int, topic uint16, interval time.Duration) {
	xx := int16(rand.Int32() & 0xFFFF)
	for i := range n {
		name := fmt.Sprintf("mon{x:%04x,t:%04d,i:%02d}", xx, topic, i)
		td.tg.do(name, func(ctx context.Context) {
			conn, err := net.Dial("tcp", td.address)
			if err != nil {
				fmt.Printf("dbg: %v failed to connect: %v\n", name, err)
				return
			}
			defer conn.Close()

			m := msg{t: subMsg, topic: topic, b: true}
			if _, err := m.WriteTo(conn); err != nil {
				fmt.Printf("dbg: %v failed to subscribe: %v\n", name, err)
			}
			fmt.Printf("dbg: %v subscribed\n", name)

			sends := make(chan idtime)
			recvs := make(chan idtime)

			go func() {
				obs := make(map[int]observation)
				for {
					// fmt.Printf("dbg: %v waiting recv/send\n", name)
					select {
					case <-ctx.Done():
						return
					case x := <-sends:
						if _, ok := obs[x.i]; ok {
							fmt.Printf("dbg: %v - send after receive?!\n", name)
						} else {
							obs[x.i] = observation{send: x.t}
							// fmt.Printf("dbg: %v got send\n", name)
						}
					case x := <-recvs:
						if o, ok := obs[x.i]; !ok {
							fmt.Printf("dbg: %v - receive without send?!\n", name)
						} else {
							o.recv = x.t
							// fmt.Printf("dbg: %v got recv, sending to td.mobc\n", name)
							select {
							case <-ctx.Done():
							case td.mobc <- monitorObservation{topic: topic, send: o.send, recv: o.recv}:
								// fmt.Printf("dbg: %v sent observation\n", name)
							}
						}
					}
				}
			}()

			prefix := fmt.Sprintf("%v ", name)

			go func() {
				for {
					// fmt.Printf("dbg: %v reading message\n", name)
					m := msg{}
					if _, err := m.ReadFrom(conn); err != nil {
						if err != io.EOF {
							// fmt.Printf("dbg: %v failed to read: %v\n", name, err)
						}
						return
					}
					// fmt.Printf("dbg: %v read message\n", name)
					t := time.Now()
					if m.t == pubMsg && m.topic == topic && strings.Index(m.payload, prefix) == 0 {
						s := m.payload[len(prefix):]
						i, err := strconv.ParseInt(s, 10, 32)
						if err != nil {
							fmt.Printf("dbg: %v unable to parse %q: %v\n", name, s, err)
							return
						}
						// fmt.Printf("dbg: %v read - sending recv time\n", name)
						select {
						case <-ctx.Done():
						case recvs <- idtime{int(i), t}:
							// fmt.Printf("dbg: %v read - sent recv time\n", name)
						}
					}
				}
			}()

			tick := time.Tick(interval)
			check := func() bool {
				select {
				case <-ctx.Done():
					return false
				case <-tick:
					// fmt.Printf("dbg: %v write - tick\n", name)
					return true
				}
			}

			for j := 0; check(); j++ {
				payload := fmt.Sprintf("%s%d", prefix, j)
				m := msg{t: pubMsg, topic: topic, payload: payload}
				// fmt.Printf("dbg: %v writing\n", name)
				if _, err := m.WriteTo(conn); err != nil {
					fmt.Printf("dbg: %v failed to publish: %v\n", name, err)
					return
				}
				t := time.Now()
				// fmt.Printf("dbg: %v write - sending send time\n", name)
				select {
				case <-ctx.Done():
				case sends <- idtime{i: j, t: t}:
					// fmt.Printf("dbg: %v write - sent send time\n", name)
				}
			}
		})
	}
}

func (td *testDriver) spawnSubs(n int, topic uint16) {
	xx := int16(rand.Int32() & 0xFFFF)
	for i := range n {
		name := fmt.Sprintf("sub{x:%04x,t:%04d,i:%04d}", xx, topic, i)
		td.tg.do(name, func(ctx context.Context) {
			conn, err := net.Dial("tcp", td.address)
			if err != nil {
				fmt.Printf("dbg: %v failed to connect: %v\n", name, err)
				return
			}
			defer conn.Close()

			m := msg{t: subMsg, topic: topic}
			if _, err := m.WriteTo(conn); err != nil {
				fmt.Printf("dbg: %v failed to subscribe: %v\n", name, err)
				return
			}

			go io.Copy(io.Discard, conn)

			tick := time.Tick(50 * time.Second)
			for {
				select {
				case <-ctx.Done():
					return
				case <-tick:
					m := msg{t: pingMsg}
					if _, err := m.WriteTo(conn); err != nil {
						fmt.Printf("dbg: %v failed to ping: %v\n", name, err)
						return
					}
				}
			}
		})
	}
}

func (td *testDriver) spawnPubs(n int, topic uint16, interval time.Duration, payloadf func() string) {
	xx := int16(rand.Int32() & 0xFFFF)
	for i := range n {
		name := fmt.Sprintf("pub{x:%04x,t:%04d,i:%02d}", xx, topic, i)
		td.tg.do(name, func(ctx context.Context) {
			conn, err := net.Dial("tcp", td.address)
			if err != nil {
				fmt.Printf("dbg: %v failed to connect: %v\n", name, err)
				return
			}
			defer conn.Close()

			// NOTE: in principle, we don't really need to do this. we don't subscribe to any topic,
			// so we won't receive anything from the server.
			go io.Copy(io.Discard, conn)

			tick := time.Tick(interval)
			for {
				select {
				case <-ctx.Done():
					return
				case <-tick:
					p := payloadf()
					m := msg{t: pubMsg, topic: topic, payload: p}
					if _, err := m.WriteTo(conn); err != nil {
						fmt.Printf("dbg: %v failed to publish: %v\n", name, err)
					}
				}
			}
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
	td.tg.wait()

	for _, mob := range td.mobs {
		fmt.Printf("result: topic %6d: %s: delay of %v\n", mob.topic, mob.send.Format(time.TimeOnly), mob.recv.Sub(mob.send))
	}
}

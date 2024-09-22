package main

import (
	"context"
	"fmt"
	"math"
	"net"
	"sync"
	"time"
)

// TODO: include malicious users in tests scenarios!!
// e.g. responds fast to ping, but takes just long enough to receive other messages that it blocks some server goroutine but not enough to disconnect it

func testRateLimiting(address string) error {
	rawconn, err := net.Dial("tcp", address)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	pc := makeconn(ctx, cancel).
		start(rawconn, rawconn)

	joinm := protomes{t: mjoin, room: 1234, name: "test"}
	text := "hello"

	if !pc.send(joinm) {
		return fmt.Errorf("unable to join")
	}

	errc := make(chan error)

	const N = 100

	durs := make([]int64, 0, N)

	wg := new(sync.WaitGroup)

	wg.Add(1)
	go func() {
		defer wg.Done()
		select {
		case <-pc.ctx.Done():
			errc <- fmt.Errorf("unable to receive jned")
		case m := <-pc.in:
			ok := true &&
				m.t == mjned &&
				m.room == joinm.room &&
				m.name == joinm.name
			if !ok {
				errc <- fmt.Errorf("received unexpected message (0) %v", m)
			}
		}

		i := 0
		for i < N {
			if i%10 == 0 {
				// allow tokens to refill
				time.Sleep(800 * time.Millisecond)
			}
			s := time.Now()
			select {
			case <-pc.ctx.Done():
				errc <- fmt.Errorf("unable to receive hear")
			case m := <-pc.in:
				if m.t == mping {
					if !pc.send(protomes{t: mpong}) {
						errc <- fmt.Errorf("failed to pong back")
					}
				} else {
					ok := true &&
						m.t == mhear &&
						m.room == joinm.room &&
						m.name == joinm.name &&
						m.text == text
					if !ok {
						errc <- fmt.Errorf("received unexpected message (1) %v", m)
					} else {
						durs = append(durs, int64(time.Since(s)))
						i++
					}
				}
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for range N {
			m := protomes{t: mtalk, room: joinm.room, text: text}
			if !pc.send(m) {
				errc <- fmt.Errorf("unable to talk")
			}
		}
	}()

	done := make(chan zero)
	go func() {
		defer close(done)
		wg.Wait()
	}()

	select {
	case err := <-errc:
		return err
	case <-done:
	}

	sum := int64(0)
	for _, dur := range durs {
		sum += dur
	}
	avg := sum / int64(len(durs))

	ss := int64(0)
	for _, dur := range durs {
		dev := dur - avg
		ss += dev * dev
	}
	stdev := math.Sqrt(float64(ss) / N)

	for _, dur := range durs {
		fmt.Printf("%20v\n", time.Duration(dur))
	}

	fmt.Println()
	fmt.Println("avg:   ", time.Duration(avg))
	fmt.Println("stdev: ", time.Duration(stdev))

	return nil
}

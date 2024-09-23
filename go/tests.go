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

	select {
	case <-pc.ctx.Done():
		return fmt.Errorf("unable to join")
	case pc.out <- joinm:
	}

	errc := make(chan error)

	const N = 200

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
			if i%20 == 0 {
				// allow tokens to refill
				time.Sleep(800 * time.Millisecond)
			}
			if i%35 == 0 {
				time.Sleep(1600 * time.Millisecond)
			}
			s := time.Now()
			select {
			case <-pc.ctx.Done():
				errc <- fmt.Errorf("unable to receive hear")
			case m := <-pc.in:
				if m.t == mping {
					select {
					case <-pc.ctx.Done():
						errc <- fmt.Errorf("failed to pong back")
					case pc.out <- protomes{t: mpong}:
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
						dur := time.Since(s)
						fmt.Printf("%4d %20v\n", i, dur)
						durs = append(durs, int64(dur))
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
			select {
			case <-pc.ctx.Done():
				errc <- fmt.Errorf("unable to talk")
			case pc.out <- m:
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

	fmt.Println("avg:   ", time.Duration(avg))
	fmt.Println("stdev: ", time.Duration(stdev))

	return nil
}

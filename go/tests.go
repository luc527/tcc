package main

import (
	"context"
	"fmt"
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

	sendtimes := make([]time.Time, N)
	recvtimes := make([]time.Time, N)

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

		for i := range N {
			select {
			case <-pc.ctx.Done():
				errc <- fmt.Errorf("unable to receive hear [%d]", i)
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
						recvtimes[i] = time.Now()
					}
				}
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := range N {
			if !pc.send(protomes{t: mtalk, room: joinm.room, text: text}) {
				errc <- fmt.Errorf("unable to talk")
			} else {
				sendtimes[i] = time.Now()
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

	durs := make([]time.Duration, N)
	sum := int64(0)
	for i := range N {
		dur := recvtimes[i].Sub(sendtimes[i])
		durs[i] = dur
		sum += int64(dur)
	}
	avg := time.Duration(sum / int64(len(durs)))
	fmt.Println("avg:", avg)

	return nil
}

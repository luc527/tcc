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

func t_recvchan(id int, wg *sync.WaitGroup, pc protoconn, ms <-chan protomes, pong chan<- zero, errc chan<- error) {
	defer func() {
		fmt.Printf("%d - recv done\n", id)
		wg.Done()
	}()
	for mwant := range ms {
		received := false
		for !received {
			select {
			case <-pc.ctx.Done():
				errc <- fmt.Errorf("%d) failed to receive", id)
				return
			case mgot, ok := <-pc.in:
				if !ok {
					return
				} else if mgot.t == mping {
					pong <- zero{}
				} else if !mgot.equal(mwant) {
					gots := mgot.String()
					if mgot.t == mprob {
						gots = "[" + ecode(mgot.room).String() + "]"
					}
					errc <- fmt.Errorf("%d) wanted to receive %v but got %v", id, mwant, gots)
					return
				} else {
					received = true
				}
			}
		}
	}
}

func t_sendchan(id int, wg *sync.WaitGroup, pc protoconn, ms <-chan protomes, pong <-chan zero, errc chan<- error) {
	defer func() {
		fmt.Printf("%d - send done\n", id)
		wg.Done()
	}()
	for m := range ms {
		sent := false
		for !sent {
			select {
			case <-pong:
				select {
				case <-pc.ctx.Done():
					errc <- fmt.Errorf("%d) failed to pong", id)
					return
				case pc.out <- protomes{t: mpong}:
				}
			case <-pc.ctx.Done():
				errc <- fmt.Errorf("%d) failed to send %v", id, m)
				return
			case pc.out <- m:
				sent = true
			}
		}
	}
}

func t_start(id int, wg *sync.WaitGroup, address string, errc chan<- error) (send chan protomes, recv chan protomes) {
	rawconn, err := net.Dial("tcp", address)
	if err != nil {
		errc <- err
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	pc := makeconn(ctx, cancel).start(rawconn, nil)
	pong := make(chan zero)
	send, recv = make(chan protomes), make(chan protomes)

	wg.Add(1)
	go t_sendchan(id, wg, pc, send, pong, errc)

	wg.Add(1)
	go t_recvchan(id, wg, pc, recv, pong, errc)

	return
}

func testFunctional(address string) error {
	done := make(chan zero)
	errc := make(chan error)

	wg := new(sync.WaitGroup)

	// TODO: if you run this test once, it goes fine
	// but if you try it again shortly after, it fails
	// and if you try it many times in a row, you get a transient error
	// check what's going on the server
	// maybe part of the problem is that not all the mprob messages are being sent back correctly?

	send1, recv1 := t_start(1, wg, address, errc)
	send2, recv2 := t_start(2, wg, address, errc)
	send3, recv3 := t_start(3, wg, address, errc)

	go func() {
		tick := time.Tick(32 * time.Millisecond)
		room := uint32(1234)

		name1 := "testman"
		send1 <- protomes{t: mjoin, room: room, name: name1}
		recv1 <- protomes{t: mjned, room: room, name: name1}
		<-tick

		text := "hello good morning"
		send1 <- protomes{t: mtalk, room: room, text: text}
		recv1 <- protomes{t: mhear, room: room, name: name1, text: text}
		<-tick

		name2 := "name"
		send2 <- protomes{t: mjoin, room: room, name: name2}
		recv2 <- protomes{t: mjned, room: room, name: name2}
		recv1 <- protomes{t: mjned, room: room, name: name2}
		<-tick

		text = "olá bom dia"
		send2 <- protomes{t: mtalk, room: room, text: text}
		recv2 <- protomes{t: mhear, room: room, name: name2, text: text}
		recv1 <- protomes{t: mhear, room: room, name: name2, text: text}
		<-tick

		name3 := "abcd"
		send3 <- protomes{t: mjoin, room: room, name: name3}
		recv3 <- protomes{t: mjned, room: room, name: name3}
		recv2 <- protomes{t: mjned, room: room, name: name3}
		recv1 <- protomes{t: mjned, room: room, name: name3}
		<-tick

		text = "wow"
		send1 <- protomes{t: mtalk, room: room, text: text}
		recv3 <- protomes{t: mhear, room: room, name: name1, text: text}
		recv2 <- protomes{t: mhear, room: room, name: name1, text: text}
		recv1 <- protomes{t: mhear, room: room, name: name1, text: text}
		<-tick

		send2 <- protomes{t: mexit, room: room}
		recv3 <- protomes{t: mexed, room: room, name: name2}
		recv1 <- protomes{t: mexed, room: room, name: name2}
		close(send2)
		<-tick
		close(recv2)

		text = "well goodbye"
		send3 <- protomes{t: mtalk, room: room, text: text}
		recv3 <- protomes{t: mhear, room: room, name: name3, text: text}
		<-tick

		send3 <- protomes{t: mexit, room: room}
		close(send3)
		<-tick
		close(recv3)

		recv1 <- protomes{t: mhear, room: room, name: name3, text: text}
		close(send1)
		<-tick
		close(recv1)
	}()

	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case err := <-errc:
		return err
	}
}

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

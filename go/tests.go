package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"net"
	"sync"
	"time"
)

// TODO: include malicious users in (performance) test scenarios!!
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
				case pc.out <- pongmes():
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

func test1(address string) error {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	pc := makeconn(ctx, cancel).start(conn, nil)
	tick := time.Tick(1 * time.Second)

	mtypes := []mtype{mjoin, mtalk, mtalk, mjoin, mtalk, mexit}
	mtypei := 0

	rooms := []uint32{123, 123, 234, 456, 456, 123}
	roomi := 0

	names := []string{"tes", "teste", "te"}
	namei := 0

	texts := []string{"hello", "Hello again", "Change da world. My final message.", "goodb ye"}
	texti := 0

	go func() {
		for {
			select {
			case <-pc.ctx.Done():
			case m := <-pc.in:
				log.Println("received", m)
				// if m.t == mping {
				// 	log.Println("sending pong back")
				// 	pc.send(pongmes())
				// }
			}
		}
	}()

	for range 20 {
		mtype := mtypes[mtypei]
		room := rooms[roomi]
		name := names[namei]
		text := texts[texti]

		m := protomes{t: mtype, room: room, name: name, text: text}
		log.Println("sending", m)
		pc.send(m)

		namei = (namei + 1) % len(names)
		roomi = (roomi + 1) % len(rooms)
		mtypei = (mtypei + 1) % len(mtypes)
		texti = (texti + 1) % len(texts)
		<-tick
	}
	return nil
}

func testTimeout(address string) error {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	pc := makeconn(ctx, cancel).start(conn, nil)

	// TODO: not working!

	select {
	case <-time.After(45 * time.Second):
		return fmt.Errorf("didn't timeout")
	case <-pc.ctx.Done():
		return nil
	}
}

func testSingleRoom(address string) error {
	done := make(chan zero)
	errc := make(chan error)

	wg := new(sync.WaitGroup)

	send1, recv1 := t_start(1, wg, address, errc)
	send2, recv2 := t_start(2, wg, address, errc)
	send3, recv3 := t_start(3, wg, address, errc)

	go func() {
		tick := time.Tick(32 * time.Millisecond)
		room := uint32(1234)

		name1 := "testman"
		send1 <- joinmes(room, name1)
		recv1 <- jnedmes(room, name1)
		<-tick

		text := "hello good morning"
		send1 <- talkmes(room, text)
		recv1 <- hearmes(room, name1, text)
		<-tick

		name2 := "name"
		send2 <- joinmes(room, name2)
		recv2 <- jnedmes(room, name2)
		recv1 <- jnedmes(room, name2)
		<-tick

		text = "olá bom dia"
		send2 <- talkmes(room, text)
		recv2 <- hearmes(room, name2, text)
		recv1 <- hearmes(room, name2, text)
		<-tick

		// TODO: test joining another room and talking there too

		name3 := "abcd"
		send3 <- joinmes(room, name3)
		recv3 <- jnedmes(room, name3)
		recv2 <- jnedmes(room, name3)
		recv1 <- jnedmes(room, name3)
		<-tick

		text = "wow"
		send1 <- talkmes(room, text)
		recv3 <- hearmes(room, name1, text)
		recv2 <- hearmes(room, name1, text)
		recv1 <- hearmes(room, name1, text)
		<-tick

		send2 <- exitmes(room)
		recv3 <- exedmes(room, name2)
		recv1 <- exedmes(room, name2)
		close(send2)
		<-tick
		close(recv2)

		text = "well goodbye"
		send3 <- talkmes(room, text)
		recv3 <- hearmes(room, name3, text)
		<-tick

		send3 <- exitmes(room)
		close(send3)
		<-tick
		close(recv3)

		recv1 <- hearmes(room, name3, text)
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

// TODO: automate rate limiting test

func testRateLimiting(address string) error {
	rawconn, err := net.Dial("tcp", address)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	pc := makeconn(ctx, cancel).
		start(rawconn, rawconn)

	joinm := joinmes(1234, "test")
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
					case pc.out <- pongmes():
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
			m := talkmes(joinm.room, text)
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

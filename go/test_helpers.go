package main

import (
	"net"
	"sync"
)

func multiconnect(r []net.Conn, n int, p int, address string) []net.Conn {
	if r == nil {
		r = make([]net.Conn, 0, n)
	}

	work := make(chan zero)
	ch := make(chan net.Conn)

	go func() {
		for range n {
			work <- zero{}
		}
		close(work)
	}()

	wg := new(sync.WaitGroup)
	for range p {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range work {
				conn, err := net.Dial("tcp", address)
				if err != nil {
					dbg("multiconnect: %v", err)
				}
				ch <- conn
			}
		}()
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	for c := range ch {
		r = append(r, c)
	}

	return r
}

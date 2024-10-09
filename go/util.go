package main

import (
	"io"
	"regexp"
	"time"
)

type zero = struct{}

type waitcloser struct {
	d time.Duration
	c io.Closer
}

var _ io.Closer = waitcloser{}

func (wc waitcloser) Close() error {
	time.Sleep(wc.d)
	return wc.c.Close()
}

var respace = regexp.MustCompile(`\s+`)

// goroutine count, for trying to detect goroutine leaks
// var gocount = atomic.Int32{}

// TODO: check for goroutine leaks again

func goinc() {
	// log.Printf("<go> count: %d", gocount.Add(1))
}

func godec() {
	// log.Printf("<go> count: %d", gocount.Add(-1))
}

func runmiddleware(dest chan<- protomes, src <-chan protomes, done <-chan zero, d dir, f func(protomes, dir)) {
	for {
		select {
		case v := <-src:
			f(v, d)
			select {
			case dest <- v:
			case <-done:
				return
			}
		case <-done:
			return
		}
	}
}

func readfull(r io.Reader, destination []byte) (n int, err error) {
	remaining := destination
	for len(remaining) > 0 {
		if nn, err := r.Read(remaining); err != nil {
			return n, err
		} else {
			n += nn
			remaining = remaining[nn:]
		}
	}
	return n, err
}

func trysend[T any](dest chan<- T, v T, done <-chan zero) bool {
	select {
	case dest <- v:
		return true
	case <-done:
		return false
	}
}

// think of the ratelimitee [sic] as needing a token to perform some action
// it can accumulate at most `b` tokens, so if it wishes it can perform the action `b` times in a (b)urst
// but it gains a new token at the (r)ate of `r`, i.e. only every `r` time units
func ratelimiter(done <-chan zero, r time.Duration, b int) <-chan zero {
	toks := make(chan zero, b)
	for range b {
		toks <- zero{}
	}
	go func() {
		t := time.Tick(r)
		for range t {
			select {
			case <-done:
				return
			case toks <- zero{}:
			}
		}
	}()
	return toks
}

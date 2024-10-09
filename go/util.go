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

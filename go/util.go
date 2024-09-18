package main

import (
	"io"
	"regexp"
	"sync/atomic"
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
// TODO: test console.go too
var gocount = atomic.Int32{}

func goinc() {
	// log.Printf("<go> count: %d", gocount.Add(1))
}

func godec() {
	// log.Printf("<go> count: %d", gocount.Add(-1))
}

func runmiddleware[T any](dest chan<- T, src <-chan T, done <-chan zero, f func(T)) {
	for {
		select {
		case v := <-src:
			f(v)
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

// dumb implementation of a queue
type queue[T any] []T

func newqueue[T any]() *queue[T] {
	var s []T
	q := queue[T](s)
	return &q
}

func (q *queue[T]) enqueue(x T) {
	*q = append(*q, x)
}

func (q *queue[T]) dequeue() T {
	defer func() {
		*q = (*q)[1:]
	}()
	return (*q)[0]
}

func (q *queue[T]) empty() bool {
	return len(*q) == 0
}

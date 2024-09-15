package main

import (
	"io"
	"regexp"
	"sync/atomic"
)

var respace = regexp.MustCompile(`\s+`)

// goroutine count, for debugging
// TODO: test console.go too
var gocount = atomic.Int32{}

func init() {
	gocount.Store(0)
}

func goinc() {
	// log.Printf("<go> count: %d", gocount.Add(1))
}

func godec() {
	// log.Printf("<go> count: %d", gocount.Add(-1))
}

type zero = struct{}

type freer[T any] struct {
	done <-chan zero
	c    chan<- T
	id   T
}

type sender[T any] struct {
	done <-chan zero
	c    chan<- T
	cf   func()
}

type receiver[T any] struct {
	done <-chan zero
	c    <-chan T
}

func (f freer[T]) free() {
	select {
	case <-f.done:
	case f.c <- f.id:
	}
}

func (s sender[T]) send(v T) {
	select {
	case <-s.done:
	case s.c <- v:
	}
}

func (s sender[T]) close() {
	s.cf()
}

func (r receiver[T]) receive() (v T, b bool) {
	select {
	case <-r.done:
	case v = <-r.c:
		b = true
	}
	return
}

func senderreceiver[T any](done <-chan zero, c chan T, cf func()) (sender[T], receiver[T]) {
	s := sender[T]{
		done: done,
		c:    c,
		cf:   cf,
	}
	r := receiver[T]{
		done: done,
		c:    c,
	}
	return s, r
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

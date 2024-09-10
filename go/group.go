package main

import "sync"

// spawn goroutines and wait for them to complete
// mostly for testing

type group struct {
	*sync.WaitGroup
}

func newg() group {
	return group{new(sync.WaitGroup)}
}

func (g group) r(f func()) {
	g.Add(1)
	go func() {
		defer g.Done()
		f()
	}()
}

func (g group) w() {
	g.Wait()
}

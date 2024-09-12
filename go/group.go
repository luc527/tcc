package main

import "sync"

// spawn goroutines and wait for them to complete
// mostly for testing

type group struct {
	*sync.WaitGroup
}

func newgroup() group {
	return group{new(sync.WaitGroup)}
}

func (g group) run(f func()) {
	g.Add(1)
	go func() {
		defer g.Done()
		f()
	}()
}

func (g group) wait() {
	g.Wait()
}

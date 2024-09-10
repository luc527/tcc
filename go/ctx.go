package main

import "context"

// TODO: embed in other structs with the same behaviour (client, hub, room, roomhandle?)

type ctx struct {
	c context.Context
	f context.CancelFunc
}

func (c ctx) done() <-chan zero {
	return c.c.Done()
}

func (c ctx) cancel() {
	select {
	case <-c.done():
	default:
		c.f()
	}
}

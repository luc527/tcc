package main

import "context"

type ctx struct {
	c context.Context
	f context.CancelFunc
}

func makectx(parent context.Context) ctx {
	c, f := context.WithCancel(parent)
	return ctx{c, f}
}

func backgroundctx() ctx {
	c, f := context.WithCancel(context.Background())
	return ctx{c, f}
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

func (c ctx) makechild() ctx {
	return makectx(c.c)
}
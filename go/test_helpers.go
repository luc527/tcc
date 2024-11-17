package main

import (
	"context"
	"net"
	"sync"
	"time"
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

type ctxconn struct {
	ctx    context.Context
	cancel context.CancelFunc
	conn   net.Conn
}

type connpool struct {
	address string
	prepare func(context.Context, net.Conn)
	parent  context.Context
	a       []ctxconn
	i       int
}

func newConnpool(parent context.Context, n int, address string, prepare func(context.Context, net.Conn)) *connpool {
	return &connpool{
		address: address,
		parent:  parent,
		prepare: prepare,
		a:       make([]ctxconn, 0, n),
		i:       0,
	}
}

func (cp *connpool) connect(r []ctxconn, n int, p int) []ctxconn {
	if r == nil {
		r = make([]ctxconn, 0, n)
	}

	if len(cp.a) == cap(cp.a) {
		for range n {
			r = append(r, cp.a[cp.i])
			cp.i = (cp.i + 1) % len(cp.a)
		}
		return r
	}

	available := cap(cp.a) - len(cp.a)
	if n <= available {
		created := multiconnect(nil, n, p, cp.address)
		for _, conn := range created {
			ctx, cancel := context.WithCancel(cp.parent)
			if conn != nil {
				cp.prepare(ctx, conn)
			}
			cc := ctxconn{ctx, cancel, conn}
			cp.a = append(cp.a, cc)
		}
		return cp.a[len(cp.a)-n:]
	}

	r = cp.connect(r, available, p)
	r = cp.connect(r, n-available, p)
	return r
}

type throughputFrame struct {
	size  time.Duration
	start time.Time
	sent  int
	index int
}

type throughputMeasurement struct {
	i int
	v float64 // messages sent per second
}

func newThroughputFrame(size time.Duration) *throughputFrame {
	tf := &throughputFrame{
		size:  size,
		start: time.Now(),
		sent:  0,
		index: 0,
	}
	return tf
}

func (tf *throughputFrame) onsend() throughputMeasurement {
	tf.sent++
	elapsed := time.Since(tf.start)
	if elapsed == 0 {
		return throughputMeasurement{i: tf.index, v: 0}
	}
	throughput := float64(tf.sent) / elapsed.Seconds()
	measurement := throughputMeasurement{i: tf.index, v: throughput}
	if elapsed > tf.size {
		tf.start = time.Now()
		tf.sent = 0
		tf.index++
	}
	return measurement
}

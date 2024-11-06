package main

import (
	"context"
	"io"
	"iter"
	"log"
	"net"
	"runtime"
	"time"
)

const (
	readTimeout  = 1 * time.Minute
	writeTimeout = 10 * time.Second
)

var (
	numPartitions = runtime.NumCPU()
)

func serve(l net.Listener, sv server) {
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		go handleConn(sv, conn)
	}
}

func messages(done <-chan zero, in <-chan msg) iter.Seq[msg] {
	return func(yield func(msg) bool) {
		for {
			select {
			case <-done:
				return
			case m := <-in:
				if !yield(m) {
					return
				}
			}
		}
	}
}

func handleConn(sv server, conn net.Conn) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := subscriber{
		done: ctx.Done(),
		sub:  make(chan subscription),
		pub:  make(chan publication),
	}

	context.AfterFunc(ctx, func() {
		sv.disconnect(s)
	})

	ping := make(chan zero)
	go writeToConn(ctx, cancel, s, ping, conn)

	in := make(chan msg)
	go readFromConn(ctx, cancel, in, conn)

	for m := range messages(ctx.Done(), in) {
		switch m.t {
		case pingMsg:
			select {
			case <-ctx.Done():
				return
			case ping <- zero{}:
			}
		case pubMsg:
			sv.publish(m.topic, m.payload)
		case subMsg:
			sv.subscribe(m.topic, s, m.b)
		}
	}
}

func writeToConn(
	ctx context.Context,
	cancel context.CancelFunc,
	s subscriber,
	ping <-chan zero,
	conn net.Conn,
) {
	defer cancel()
	defer func() {
		if err := conn.Close(); err != nil {
			log.Printf("failed to close: %v", err)
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ping:
			m := msg{
				t: pingMsg,
			}
			if !tryWrite(conn, m) {
				return
			}
		case sx := <-s.sub:
			m := msg{
				t:     subMsg,
				topic: sx.topic,
				b:     sx.subscribed,
			}
			if !tryWrite(conn, m) {
				return
			}
		case px := <-s.pub:
			m := msg{
				t:       pubMsg,
				topic:   px.topic,
				payload: px.payload,
			}
			if !tryWrite(conn, m) {
				return
			}
		}
	}
}

func tryWrite(conn net.Conn, m msg) bool {
	if err := conn.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
		log.Printf("failed to set write deadline: %v", err)
		return false
	}
	if _, err := m.WriteTo(conn); err != nil {
		log.Printf("failed to write %v: %v", m, err)
		return false
	}
	return true
}

func readFromConn(ctx context.Context, cancel context.CancelFunc, in chan<- msg, conn net.Conn) {
	defer cancel()
	for {
		if err := conn.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
			log.Printf("failed to set read deadline: %v", err)
		}
		var m msg
		if _, err := m.ReadFrom(conn); err != nil {
			if err != io.EOF {
				log.Printf("failed to read: %v", err)
			}
			return
		} else {
			select {
			case <-ctx.Done():
				return
			case in <- m:
			}
		}
	}
}

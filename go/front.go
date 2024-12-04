package main

import (
	"bytes"
	"context"
	"io"
	"log"
	"net"
	"runtime"
	"time"
)

type zero = struct{}

const (
	readTimeout = 1 * time.Minute
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
		go handleConn(conn, sv)
	}
}

func handleConn(conn net.Conn, sv server) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mc := make(chan msg, 1)
	s := subscriber{
		done: ctx.Done(),
		mc:   mc,
	}

	context.AfterFunc(ctx, func() {
		if err := conn.Close(); err != nil {
			log.Printf("error when closing connection: %v", err)
		}
		sv.disconnect(s)
	})

	go writeToConn(ctx.Done(), mc, conn)

	for {
		if s.isDone() {
			return
		}

		m := msg{}
		if err := conn.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
			log.Printf("failed to set read deadline: %v", err)
			return
		}
		if _, err := m.ReadFrom(conn); err != nil {
			if err != io.EOF {
				log.Printf("failed to read message: %v", err)
			}
			return
		}
		switch m.t {
		case pingMsg:
			m := msg{t: pingMsg}
			if _, err := m.WriteTo(conn); err != nil {
				log.Printf("failed to ping back: %v\n", err)
				return
			}
		case pubMsg:
			sv.publish(m.topic, m.payload)
		case subMsg:
			sv.subscribe(m.topic, s, true)
		case unsubMsg:
			sv.subscribe(m.topic, s, false)
		}
	}
}

func writeToConn(done <-chan zero, mc <-chan msg, conn net.Conn) {
	buf := new(bytes.Buffer)
	for {
		select {
		case <-done:
			return
		case m := <-mc:
			if _, err := m.WriteTo(buf); err != nil {
				log.Printf("failed to write message to buffer: %v", err)
				return
			}
			if _, err := buf.WriteTo(conn); err != nil {
				log.Printf("failed to write message to the connection: %v", err)
				return
			}
		}
	}
}

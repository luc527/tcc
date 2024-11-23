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

type clientconn struct {
	done <-chan zero
	conn net.Conn
}

func (cc *clientconn) isDone() bool {
	select {
	case <-cc.done:
		return true
	default:
		return false
	}
}

func (cc *clientconn) sendUnbuffered(m msg) {
	select {
	case <-cc.done:
		return
	default:
	}
	if _, err := m.WriteTo(cc.conn); err != nil {
		log.Printf("failed to send message %v to client: %v", m, err)
		return
	}
}

func (cc *clientconn) send(m msg, buf *bytes.Buffer) {
	select {
	case <-cc.done:
		return
	default:
	}
	buf.Reset()
	if _, err := m.WriteTo(buf); err != nil {
		log.Printf("failed to write message %v to given buffer: %v", m, err)
		return
	}
	if _, err := buf.WriteTo(cc.conn); err != nil {
		log.Printf("failed to send message %v to client: %v", m, err)
		return
	}
}

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

	cc := &clientconn{
		done: ctx.Done(),
		conn: conn,
	}

	context.AfterFunc(ctx, func() {
		if err := conn.Close(); err != nil {
			log.Printf("error when closing connection: %v", err)
		}
		sv.disconnect(cc)
	})

	for {
		if cc.isDone() {
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
			cc.sendUnbuffered(msg{t: pingMsg})
		case pubMsg:
			sv.publish(m.topic, m.payload)
		case subMsg:
			sv.subscribe(m.topic, cc, true)
		case unsubMsg:
			sv.subscribe(m.topic, cc, false)
		}
	}
}

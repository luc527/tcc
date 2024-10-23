package main

import (
	"context"
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

type server struct {
	partitions []*serverPartition
}

// NOTE: since the partitions slice won't be modified, we don't need server methods to have ptr
// receivers, as in (s *server)

func newServer(numPartitions int) (server, error) {
	s := server{
		partitions: make([]*serverPartition, numPartitions),
	}
	for id := range numPartitions {
		sp := newServerPartition(id)
		s.partitions[id] = sp
	}
	return s, nil
}

func (s server) partition(topic uint16) *serverPartition {
	return s.partitions[int(topic)%len(s.partitions)]
}

func (s server) sub(topic uint16, sub subscriber) bool {
	sp := s.partition(topic)
	return sp.sub(topic, sub)
}

func (s server) unsub(topic uint16, sub subscriber) bool {
	sp := s.partition(topic)
	return sp.unsub(topic, sub)
}

func (s server) pub(topic uint16, payload string) bool {
	sp := s.partition(topic)
	return sp.pub(topic, payload)
}

func (s server) start() {
	for _, sp := range s.partitions {
		sp.start()
	}
}

func serve(l net.Listener, s server) {
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		go handleConn(s, conn)
	}
}

func handleConn(s server, conn net.Conn) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // overkill?

	sub := subscriber{
		subbedc: make(chan subbed),
		pubc:    make(chan publication),
		done:    ctx.Done(),
	}
	ping := make(chan zero)
	go writeToConn(ctx, cancel, sub, ping, conn)

	in := make(chan msg)
	go readFromConn(ctx, cancel, in, conn)

	for {
		select {
		case <-ctx.Done():
			return
		case m := <-in:
			if m.t == pingMsg {
				select {
				case <-ctx.Done():
					return
				case ping <- zero{}:
				}
			} else {
				handleMessage(s, sub, m)
			}
		}
	}
}

func handleMessage(s server, sub subscriber, m msg) {
	switch m.t {
	case pubMsg:
		s.pub(m.topic, m.payload)
	case subMsg:
		s.sub(m.topic, sub)
	case unsubMsg:
		s.unsub(m.topic, sub)
	}
}

func writeToConn(
	ctx context.Context,
	cancel context.CancelFunc,
	sub subscriber,
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
		case sed := <-sub.subbedc:
			m := msg{
				t:      subbedMsg,
				topic:  sed.topic,
				subbed: sed.b,
			}
			if !tryWrite(conn, m) {
				return
			}
		case px := <-sub.pubc:
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
			log.Printf("failed to read: %v", err)
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

package main

import (
	"fmt"
)

type zero = struct{}

type publication struct {
	topic   uint16
	payload string
}

func (p publication) String() string {
	return fmt.Sprintf("publication{topic: %d, payload: %q}", p.topic, p.payload)
}

type subbed struct {
	topic uint16
	b     bool
}

func (s subbed) String() string {
	return fmt.Sprintf("subbed{topic: %d, b: %v}", s.topic, s.b)
}

type subscriber struct {
	subbedc chan subbed
	pubc    chan publication
	done    <-chan zero
}

type subscription struct {
	topic uint16
	sub   subscriber
}

type serverPartition struct {
	id     int
	subs   map[uint16]map[subscriber]zero
	subc   chan subscription
	unsubc chan subscription
	pubc   chan publication
	done   chan zero
}

func newServerPartition(id int) *serverPartition {
	return &serverPartition{
		id:     id,
		subs:   make(map[uint16]map[subscriber]zero),
		subc:   make(chan subscription),
		unsubc: make(chan subscription),
		pubc:   make(chan publication),
		done:   make(chan zero),
	}
}

func makeSubscriber() subscriber {
	return subscriber{
		subbedc: make(chan subbed),
		pubc:    make(chan publication),
		done:    make(chan zero),
	}
}

func (sp *serverPartition) sub(topic uint16, s subscriber) bool {
	select {
	case <-sp.done:
		return false
	case sp.subc <- subscription{topic, s}:
		return true
	}
}

func (sp *serverPartition) pub(topic uint16, payload string) bool {
	select {
	case <-sp.done:
		return false
	case sp.pubc <- publication{topic, payload}:
		return true
	}
}

func (sp *serverPartition) unsub(topic uint16, s subscriber) bool {
	select {
	case <-sp.done:
		return false
	case sp.unsubc <- subscription{topic, s}:
		return true
	}

}

func (sp *serverPartition) main() {
	for {
		select {
		case s := <-sp.subc:
			sp.handleSub(s)
		case s := <-sp.unsubc:
			sp.handleUnsub(s)
		case p := <-sp.pubc:
			sp.handlePub(p)
		case <-sp.done:
			return
		}
	}
}

func (sp *serverPartition) start() {
	go sp.main()
}

func (sp *serverPartition) stop() {
	select {
	case <-sp.done:
	default:
		close(sp.done)
	}
}

func (sp *serverPartition) handleSub(sx subscription) {
	subs, ok := sp.subs[sx.topic]
	if !ok {
		subs = make(map[subscriber]zero)
		sp.subs[sx.topic] = subs
	}

	subs[sx.sub] = zero{}

	subbed := subbed{topic: sx.topic, b: true}
	select {
	case sx.sub.subbedc <- subbed:
	case <-sx.sub.done:
	}
}

func (sp *serverPartition) handleUnsub(sx subscription) {
	subs, ok := sp.subs[sx.topic]
	if !ok {
		return
	}

	delete(subs, sx.sub)

	if len(subs) == 0 {
		delete(sp.subs, sx.topic)
	}

	subbed := subbed{topic: sx.topic, b: false}
	select {
	case sx.sub.subbedc <- subbed:
	case <-sx.sub.done:
	}
}

func (sp *serverPartition) handlePub(px publication) {
	subs, ok := sp.subs[px.topic]
	if !ok {
		return
	}

	for sub := range subs {
		select {
		case sub.pubc <- px:
		case <-sub.done:
		}
	}
}

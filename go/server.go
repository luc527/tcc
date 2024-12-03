package main

import (
	"slices"
	"strings"
)

type subscriber struct {
	done <-chan zero
	mc   chan<- msg
}

func (s subscriber) send(m msg) {
	select {
	case <-s.done:
	case s.mc <- m:
	}
}

func (s subscriber) isDone() bool {
	select {
	case <-s.done:
		return true
	default:
		return false
	}
}

type serverPartition struct {
	subscribers map[uint16]map[subscriber]zero
	topics      map[subscriber]map[uint16]zero
}

func makeServerPartition() serverPartition {
	return serverPartition{
		subscribers: make(map[uint16]map[subscriber]zero),
		topics:      make(map[subscriber]map[uint16]zero),
	}
}

func (sp serverPartition) handleDisconnect(s subscriber) {
	ts, ok := sp.topics[s]
	if !ok {
		return
	}
	for t := range ts {
		if ss, ok := sp.subscribers[t]; ok {
			delete(ss, s)
			if len(ss) == 0 {
				delete(sp.subscribers, t)
			}
		}
	}
	delete(sp.topics, s)
}

func (sp serverPartition) handleSubscribe(t uint16, s subscriber) {
	ts, ok := sp.topics[s]
	if !ok {
		ts = make(map[uint16]zero)
		sp.topics[s] = ts
	}
	if _, ok := ts[t]; !ok {
		ts[t] = zero{}
		ss, ok := sp.subscribers[t]
		if !ok {
			ss = make(map[subscriber]zero)
			sp.subscribers[t] = ss
		}
		ss[s] = zero{}
	}

	m := msg{t: subMsg, topic: t}
	s.send(m)
}

func (sp serverPartition) handleUnsubscribe(t uint16, s subscriber) {
	ss, ok := sp.subscribers[t]
	if ok {
		delete(ss, s)
		if len(ss) == 0 {
			delete(sp.subscribers, t)
		}
	}

	ts, ok := sp.topics[s]
	if ok {
		delete(ts, t)
		if len(ts) == 0 {
			delete(sp.topics, s)
		}
	}

	m := msg{t: unsubMsg, topic: t}
	s.send(m)
}

func (sp serverPartition) handlePublish(t uint16, p string) {
	ss, ok := sp.subscribers[t]
	if !ok {
		return
	}
	m := msg{t: pubMsg, topic: t, payload: p}
	for s := range ss {
		s.send(m)
	}
}

type subscriptionRequest struct {
	topic uint16
	b     bool
	s     subscriber
}

type publication struct {
	topic   uint16
	payload string
}

type serverPartitionChannels struct {
	disconnect chan subscriber
	subscribe  chan subscriptionRequest
	publish    chan publication
}

func makeServerPartitionChannels() serverPartitionChannels {
	return serverPartitionChannels{
		disconnect: make(chan subscriber),
		subscribe:  make(chan subscriptionRequest),
		publish:    make(chan publication),
	}
}

func (spc serverPartitionChannels) main(sp serverPartition) {
	for {
		select {
		case s := <-spc.disconnect:
			sp.handleDisconnect(s)
		case sx := <-spc.subscribe:
			if sx.b {
				sp.handleSubscribe(sx.topic, sx.s)
			} else {
				sp.handleUnsubscribe(sx.topic, sx.s)
			}
		case px := <-spc.publish:
			prefix := "!rot13sort "
			if strings.Index(px.payload, prefix) == 0 {
				go func() {
					result := rot13sort(px.payload[len(prefix):])
					spc.publish <- publication{
						topic:   px.topic,
						payload: result,
					}
				}()
			} else {
				sp.handlePublish(px.topic, px.payload)
			}
		}
	}
}

type server struct {
	parts []serverPartition
	chans []serverPartitionChannels
}

func makeServer(nparts int) server {
	parts := make([]serverPartition, nparts)
	chans := make([]serverPartitionChannels, nparts)
	for i := range nparts {
		parts[i] = makeServerPartition()
		chans[i] = makeServerPartitionChannels()
	}
	return server{
		parts: parts,
		chans: chans,
	}
}

func (sv server) start() {
	for i := range sv.parts {
		go sv.chans[i].main(sv.parts[i])
	}
}

func (sv server) partitionChannels(t uint16) serverPartitionChannels {
	i := int(t) % len(sv.parts)
	return sv.chans[i]
}

func (sv server) disconnect(s subscriber) {
	for _, spc := range sv.chans {
		spc.disconnect <- s
	}
}

func (sv server) subscribe(t uint16, s subscriber, b bool) {
	sx := subscriptionRequest{t, b, s}
	sv.partitionChannels(t).subscribe <- sx
}

func (sv server) publish(t uint16, p string) {
	px := publication{t, p}
	sv.partitionChannels(t).publish <- px
}

func rot13sort(s string) string {
	bs := make([]byte, 0, len(s))
	for _, c := range s {
		if c >= 'a' && c <= 'z' {
			rot := byte(((c - 'a' + 13) % 26) + 'a')
			bs = append(bs, rot)
		} else if c >= 'A' && c <= 'Z' {
			rot := byte(((c - 'A' + 13) % 26) + 'A')
			bs = append(bs, rot)
		}
	}
	slices.Sort(bs)
	t := string(bs)
	// fmt.Printf("transformed\n%s\ninto\n%s\n", s, t)
	return t
}

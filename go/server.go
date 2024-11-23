package main

import (
	"bytes"
)

type serverPartition struct {
	subscribers map[uint16]map[*clientconn]zero
	topics      map[*clientconn]map[uint16]zero
	buf         *bytes.Buffer
}

func makeServerPartition() serverPartition {
	return serverPartition{
		buf:         new(bytes.Buffer),
		subscribers: make(map[uint16]map[*clientconn]zero),
		topics:      make(map[*clientconn]map[uint16]zero),
	}
}

func (sp serverPartition) sendTo(m msg, cc *clientconn) {
	// NOTE: buf is initially empty and every buf.WriteTo resets the buf, so we don't need to Reset
	// it before calling send.
	cc.send(m, sp.buf)
}

func (sp serverPartition) handleDisconnect(s *clientconn) {
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

func (sp serverPartition) handleSubscribe(t uint16, s *clientconn) {
	ts, ok := sp.topics[s]
	if !ok {
		ts = make(map[uint16]zero)
		sp.topics[s] = ts
	}
	if _, ok := ts[t]; !ok {
		ts[t] = zero{}
		ss, ok := sp.subscribers[t]
		if !ok {
			ss = make(map[*clientconn]zero)
			sp.subscribers[t] = ss
		}
		ss[s] = zero{}
	}

	m := msg{t: subMsg, topic: t}
	sp.sendTo(m, s)
}

func (sp serverPartition) handleUnsubscribe(t uint16, s *clientconn) {
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
	sp.sendTo(m, s)
}

func (sp serverPartition) handlePublish(t uint16, p string) {
	ss, ok := sp.subscribers[t]
	if !ok {
		return
	}
	m := msg{t: pubMsg, topic: t, payload: p}
	for s := range ss {
		sp.sendTo(m, s)
	}
}

type subscriptionRequest struct {
	topic uint16
	b     bool
	cc    *clientconn
}

type publication struct {
	topic   uint16
	payload string
}

type serverPartitionChannels struct {
	disconnect chan *clientconn
	subscribe  chan subscriptionRequest
	publish    chan publication
}

func makeServerPartitionChannels() serverPartitionChannels {
	return serverPartitionChannels{
		disconnect: make(chan *clientconn),
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
				sp.handleSubscribe(sx.topic, sx.cc)
			} else {
				sp.handleUnsubscribe(sx.topic, sx.cc)
			}
		case px := <-spc.publish:
			sp.handlePublish(px.topic, px.payload)
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

func (sv server) disconnect(s *clientconn) {
	for _, spc := range sv.chans {
		spc.disconnect <- s
	}
}

func (sv server) subscribe(t uint16, s *clientconn, b bool) {
	sx := subscriptionRequest{t, b, s}
	sv.partitionChannels(t).subscribe <- sx
}

func (sv server) publish(t uint16, p string) {
	px := publication{t, p}
	sv.partitionChannels(t).publish <- px
}

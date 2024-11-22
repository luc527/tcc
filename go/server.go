package main

type zero = struct{}

type publication struct {
	topic   uint16
	payload string
}

type subscription struct {
	topic      uint16
	subscribed bool
}

type subscriptionRequest struct {
	subscription
	subscriber
}

type subscriber struct {
	done <-chan zero
	pub  chan publication
	sub  chan subscription
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

	sx := subscription{
		topic:      t,
		subscribed: true,
	}
	select {
	case <-s.done:
	case s.sub <- sx:
	}
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

	sx := subscription{
		topic:      t,
		subscribed: false,
	}
	select {
	case <-s.done:
	case s.sub <- sx:
	}
}

func (sp serverPartition) handlePublish(t uint16, p string) {
	ss, ok := sp.subscribers[t]
	if !ok {
		return
	}
	px := publication{
		topic:   t,
		payload: p,
	}
	for s := range ss {
		select {
		case <-s.done:
		case s.pub <- px:
		}
	}
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
			if sx.subscribed {
				sp.handleSubscribe(sx.topic, sx.subscriber)
			} else {
				sp.handleUnsubscribe(sx.topic, sx.subscriber)
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

func (sv server) disconnect(s subscriber) {
	for _, spc := range sv.chans {
		spc.disconnect <- s
	}
}

func (sv server) subscribe(t uint16, s subscriber, b bool) {
	sx := subscriptionRequest{subscription{t, b}, s}
	sv.partitionChannels(t).subscribe <- sx
}

func (sv server) publish(t uint16, p string) {
	px := publication{t, p}
	sv.partitionChannels(t).publish <- px
}

package main

type conclient struct {
	mu   chan zero // only send after receiving a previous message
	id   string
	pc   protoconn
	join chan joinspec
	exit chan uint32
	talk chan talkspec
	lsro chan zero
}

func (c conclient) send(m protomes) {
	prf("< client %q: sending %v\n", c.id, m)
	c.pc.send(m)
}

func (c conclient) handlemessages() {
	defer c.pc.cancel()
	for {
		select {
		case <-c.pc.ctx.Done():
			return
		case js := <-c.join:
			m := protomes{t: mjoin, room: js.room, name: js.name}
			c.send(m)
			c.mu <- zero{}
		case room := <-c.exit:
			m := protomes{t: mexit, room: room}
			c.send(m)
		case ts := <-c.talk:
			m := protomes{t: mtalk, room: ts.room, text: ts.text}
			c.send(m)
			c.mu <- zero{}
		case <-c.lsro:
			m := protomes{t: mlsro}
			c.send(m)
		}
	}
}

func (c conclient) main() {
	defer c.pc.cancel()
	for {
		select {
		case <-c.pc.ctx.Done():
			return
		case m := <-c.pc.in:
			if m.t == mping {
				c.pc.send(protomes{t: mpong})
			} else {
				select {
				case <-c.mu:
				default:
				}

				prf("< client %q: received %v\n", c.id, m)
			}
		}
	}
}

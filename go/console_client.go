package main

type conclient struct {
	id   string
	pc   protoconn
	join chan joinspec
	exit chan uint32
	talk chan talkspec
}

func (c conclient) handlemessages() {
	defer c.pc.cancel()
	for {
		select {
		case <-c.pc.ctx.Done():
			return
		case js := <-c.join:
			m := protomes{t: mjoin, room: js.room, name: js.name}
			c.pc.send(m)
		case room := <-c.exit:
			m := protomes{t: mexit, room: room}
			c.pc.send(m)
		case talk := <-c.talk:
			m := protomes{t: mtalk, room: talk.room, text: talk.text}
			c.pc.send(m)
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
				prf("< client %q: received %v\n", c.id, m)
			}
		}
	}
}

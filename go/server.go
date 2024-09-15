package main

import (
	"net"
	"time"
)

type roomclient struct {
	connser sender[protomes]
	rms     chan roommes
	jned    chan string
	exed    chan string
}

func serve(listener net.Listener) {
	h := makehub()
	go h.main()
	for {
		rawconn, err := listener.Accept()
		if err != nil {
			continue
		}
		conn := makeconn(rawconn)
		go handleclient(conn, h)
	}
}

func handleclient(conn protoconn, h hub) {
	go conn.producein()
	go conn.consumeout()
	connser := conn.sender()
	go pingclient(connser)
	handlemessages(h, conn.in, connser)
}

func handlemessages(h hub, ms <-chan protomes, connser sender[protomes]) {
	defer connser.close()

	rooms := make(map[uint32]sender[string])
	exited := make(chan uint32)

	for {
		select {
		case <-connser.done:
			return
		case room := <-exited:
			delete(rooms, room)
		case m := <-ms:

			switch m.t {
			case mping:
				connser.send(protomes{t: mpong})
			case mjoin:
				if _, joined := rooms[m.room]; joined {
					// TODO: respond with error
					continue
				}
				rc := makeroomclient(connser)
				req, resp, prob := rc.makereq(m.name)
				h.join(m.room, req)
				select {
				case <-prob:
					connser.send(errormes(errJoinFailed))
				case roomser := <-resp:
					rooms[m.room] = roomser
					f := freer[uint32]{
						done: connser.done,
						c:    exited,
						id:   m.room,
					}
					go rc.main(m.room, roomser, f)
				}

			case mexit:
				if roomser, ok := rooms[m.room]; ok {
					roomser.close()
				}
			case msend:
				if roomser, ok := rooms[m.room]; ok {
					roomser.send(m.text)
				}
			case mpong:
			default:
				connser.send(errormes(errInvalidMessageType))
			}
		}
	}
}

func makeroomclient(connser sender[protomes]) roomclient {
	// TODO: buffered
	return roomclient{
		connser: connser,
		rms:     make(chan roommes),
		jned:    make(chan string),
		exed:    make(chan string),
	}
}

func (rc roomclient) makereq(name string) (req joinroomreq, resp chan sender[string], prob chan zero) {
	resp = make(chan sender[string])
	prob = make(chan zero)
	req = joinroomreq{
		name: name,
		rms:  rc.rms,
		jned: rc.jned,
		exed: rc.exed,
		resp: resp,
		prob: prob,
	}
	return
}

func (rc roomclient) main(room uint32, roomser sender[string], f freer[uint32]) {
	goinc()
	defer godec()
	defer f.free()
	defer roomser.close()

	for {
		select {
		case rm := <-rc.rms:
			m := protomes{mrecv, room, rm.name, rm.text}
			rc.connser.send(m)
		case name := <-rc.jned:
			m := protomes{mjned, room, name, ""}
			rc.connser.send(m)
		case name := <-rc.exed:
			m := protomes{mexed, room, name, ""}
			rc.connser.send(m)
		case <-rc.connser.done:
			return
		case <-roomser.done:
			return
		}
	}
}

func pingclient(s sender[protomes]) {
	goinc()
	defer godec()
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.send(protomes{t: mping})
		case <-s.done:
			return
		}
	}
}

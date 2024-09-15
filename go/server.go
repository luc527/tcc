package main

import (
	"net"
	"time"
)

type roomclient struct {
	connser sender[protomes]
	mesc    chan roommes
	jnedc   chan string
	exedc   chan string
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
	go conn.produceinc()
	go conn.consumeoutc()
	connser := conn.sender()
	go pingclient(connser)

	handlemessages(h, conn.inc, connser)
}

func handlemessages(h hub, ms <-chan protomes, connser sender[protomes]) {
	defer connser.close()

	rooms := make(map[uint32]sender[string])
	exitedc := make(chan uint32)

	for {
		select {
		case <-connser.done:
			return
		case room := <-exitedc:
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
						c:    exitedc,
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
		mesc:    make(chan roommes),
		jnedc:   make(chan string),
		exedc:   make(chan string),
	}
}

func (rc roomclient) makereq(name string) (req joinroomreq, resp chan sender[string], prob chan zero) {
	resp = make(chan sender[string])
	prob = make(chan zero)
	req = joinroomreq{
		name:  name,
		mesc:  rc.mesc,
		jnedc: rc.jnedc,
		exedc: rc.exedc,
		resp:  resp,
		prob:  prob,
	}
	return
}

func (rc roomclient) main(room uint32, roomser sender[string], f freer[uint32]) {
	defer f.free()
	defer roomser.close()

	for {
		select {
		case rmes := <-rc.mesc:
			m := protomes{mrecv, room, rmes.name, rmes.text}
			rc.connser.send(m)
		case name := <-rc.jnedc:
			m := protomes{mjned, room, name, ""}
			rc.connser.send(m)
		case name := <-rc.exedc:
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

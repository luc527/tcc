package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"regexp"
	"strconv"
)

func clientmain(address string) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		log.Printf("failed to connect: %v", err)
		return
	}

	pc := protoconn{
		ctx:  makectx(context.Background()),
		inc:  make(chan protomes),
		outc: make(chan protomes),
	}
	pc.start(conn)
	go handleserver(pc)

	respace := regexp.MustCompile(`\s+`)

	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		toks := respace.Split(sc.Text(), 3)
		if len(toks) == 0 {
			fmt.Println("! missing command")
			continue
		}
		cmd := toks[0]
		if cmd == "quit" {
			break
		}
		if len(toks) == 1 {
			fmt.Println("! missing room")
			continue
		}
		sroom := toks[1]
		iroom, err := strconv.ParseUint(sroom, 10, 32)
		if err != nil {
			log.Printf("! invalid room: %v\n", err)
			continue
		}
		room := uint32(iroom)

		switch cmd {
		case "join":
			if len(toks) == 2 {
				fmt.Println("! missing name")
				continue
			}
			name := toks[2]
			m := protomes{t: mjoin, room: room, name: name}
			if !pc.trysend(pc.outc, m) {
				log.Printf("failed to join (%v)\n", m)
				return
			}
		case "exit":
			m := protomes{t: mexit, room: room}
			if !pc.trysend(pc.outc, m) {
				log.Printf("failed to exit (%v)\n", m)
				return
			}
		case "send":
			if len(toks) == 2 {
				fmt.Println("! missing text")
				continue
			}
			text := toks[2]
			m := protomes{t: msend, room: room, text: text}
			if !pc.trysend(pc.outc, m) {
				log.Printf("failed to send (%v)\n", m)
				return
			}
		default:
			log.Printf("! unknown command %q\n! available commands are: %q, %q, %q and %q\n", cmd, "quit", "join", "exit", "send")
		}
	}

	if err := sc.Err(); err != nil {
		fmt.Println(err)
	}
}

func handleserver(pc protoconn) {
	defer pc.cancel()
	for {
		select {
		case m := <-pc.inc:
			switch m.t {
			case mping:
				log.Printf("< ping\n")
				if !pc.trysend(pc.outc, protomes{t: mpong}) {
					log.Printf("! disconnected\n")
					return
				}
			case mjned:
				log.Printf("< (%d, %v) joined\n", m.room, m.name)
			case mexed:
				log.Printf("< (%d, %v) exited\n", m.room, m.name)
			case mrecv:
				log.Printf("< (%d, %v) %v\n", m.room, m.name, m.text)
			case mprob:
				perr, ok := protoerrcode(uint8(m.room))
				s := "invalid code"
				if ok {
					s = perr.Error()
				}
				log.Printf("< (error) %v\n", s)
			}
		case <-pc.done():
			return
		}
	}
}

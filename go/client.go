package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
)

func clientmain(address string) {
	rawconn, err := net.Dial("tcp", address)
	if err != nil {
		log.Printf("failed to connect: %v", err)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	pc := makeconn(ctx, cancel).start(rawconn).
		startmiddleware(
			func(m protomes) { fmt.Printf("MW/I: %v\n", m) },
			func(m protomes) { fmt.Printf("MW/O: %v\n", m) },
		)

	go handleserver(pc)
	handlescanner(bufio.NewScanner(os.Stdin), pc)
}

func handleserver(pc protoconn) {
	goinc()
	defer godec()
	defer pc.cancel()

	for {
		for {
			select {
			case <-pc.ctx.Done():
				return
			case m := <-pc.in:
				switch m.t {
				case mping:
					log.Printf("< ping\n")
					if !pc.send(protomes{t: mpong}) {
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
			}
		}
	}
}

func handlescanner(sc *bufio.Scanner, pc protoconn) {
	defer pc.cancel()
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
			if !pc.send(m) {
				return
			}
		case "exit":
			m := protomes{t: mexit, room: room}
			if !pc.send(m) {
				return
			}
		case "send":
			if len(toks) == 2 {
				fmt.Println("! missing text")
				continue
			}
			text := toks[2]
			m := protomes{t: msend, room: room, text: text}
			if !pc.send(m) {
				return
			}
		default:
			log.Printf("! unknown command %q\n! available commands are: %q, %q, %q and %q\n", cmd, "quit", "join", "exit", "send")
		}

		if pc.isdone() {
			return
		}
	}

	if err := sc.Err(); err != nil {
		fmt.Println(err)
	}
}

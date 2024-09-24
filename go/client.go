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
	pc := makeconn(ctx, cancel).start(rawconn, rawconn)

	go handleserver(pc)
	handlescanner(bufio.NewScanner(os.Stdin), pc)
}

func handleserver(pc protoconn) {
	goinc()
	defer godec()
	defer pc.cancel()

	for {
		select {
		case <-pc.ctx.Done():
			return
		case m := <-pc.in:
			switch m.t {
			case mping:
				log.Printf("< ping\n")
				pc.send(protomes{t: mpong})
			case mjned:
				log.Printf("< (%d, %v) joined\n", m.room, m.name)
			case mexed:
				log.Printf("< (%d, %v) exited\n", m.room, m.name)
			case mhear:
				log.Printf("< (%d, %v) %v\n", m.room, m.name, m.text)
			case mrols:
				log.Printf("< room list\n%v\n", m.text)
			case mprob:
				log.Printf("< (error) %v\n", ecode(m.room))
			}
		}
	}
}

func handlescanner(sc *bufio.Scanner, pc protoconn) {
	defer pc.cancel()
	for sc.Scan() {
		toks := respace.Split(sc.Text(), 3)
		if len(toks) == 0 {
			fmt.Fprintln(os.Stderr, "! missing command")
			continue
		}
		cmd := toks[0]
		if cmd == "quit" {
			break
		}
		if len(toks) == 1 {
			fmt.Fprintln(os.Stderr, "! missing room")
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
				fmt.Fprintln(os.Stderr, "! missing name")
				continue
			}
			name := toks[2]
			m := protomes{t: mjoin, room: room, name: name}
			select {
			case <-pc.ctx.Done():
				goto end
			case pc.out <- m:
			}
		case "exit":
			m := protomes{t: mexit, room: room}
			select {
			case <-pc.ctx.Done():
				goto end
			case pc.out <- m:
			}
		case "talk":
			if len(toks) == 2 {
				fmt.Fprintln(os.Stderr, "! missing text")
				continue
			}
			text := toks[2]
			m := protomes{t: mtalk, room: room, text: text}
			select {
			case <-pc.ctx.Done():
				goto end
			case pc.out <- m:
			}
		case "ls":
			m := protomes{t: mlsro}
			select {
			case <-pc.ctx.Done():
				goto end
			case pc.out <- m:
			}
		default:
			log.Printf(
				"! unknown command %q\n! available commands are: %q, %q, %q %q and %q\n",
				cmd,
				"quit",
				"join",
				"exit",
				"talk",
				"ls",
			)
		}

		select {
		case <-pc.ctx.Done():
			return
		default:
		}
	}

end:
	if err := sc.Err(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

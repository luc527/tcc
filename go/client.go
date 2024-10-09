package main

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
)

func clientmain(address string) {
	rawconn, err := net.Dial("tcp", address)
	if err != nil {
		fmt.Printf("failed to connect: %v", err)
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

	for m := range pc.messages() {
		switch m.t {
		case mping:
			fmt.Printf("< ping\n")
			pc.send(pongmes())
		case mjned:
			fmt.Printf("< (%d, %v) joined\n", m.room, m.name)
		case mexed:
			fmt.Printf("< (%d, %v) exited\n", m.room, m.name)
		case mhear:
			fmt.Printf("< (%d, %v) %v\n", m.room, m.name, m.text)
		case mrols:
			fmt.Printf("< room list\n%v\n", m.text)
		case mprob:
			fmt.Printf("< (error) %v\n", ecode(m.room))
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
		if len(toks) == 1 && toks[0] != "ls" {
			fmt.Fprintln(os.Stderr, "! missing room")
			continue
		}
		sroom := toks[1]
		iroom, err := strconv.ParseUint(sroom, 10, 32)
		if err != nil {
			fmt.Printf("! invalid room: %v\n", err)
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
			if !pc.send(joinmes(room, name)) {
				goto end
			}
		case "exit":
			if !pc.send(exitmes(room)) {
				goto end
			}
		case "talk":
			if len(toks) == 2 {
				fmt.Fprintln(os.Stderr, "! missing text")
				continue
			}
			text := toks[2]
			m := talkmes(room, text)
			fmt.Println("the message is", m)
			if !pc.send(m) {
				goto end
			}
		case "ls":
			if !pc.send(lsromes()) {
				goto end
			}
		default:
			fmt.Printf(
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

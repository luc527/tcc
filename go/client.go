package main

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"tccgo/conn"
	"tccgo/mes"
)

func clientmain(address string) {
	rawconn, err := net.Dial("tcp", address)
	if err != nil {
		fmt.Printf("failed to connect: %v", err)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	c := conn.New(ctx, cancel)
	c.Start(rawconn)

	go handleserver(c)
	handlescanner(bufio.NewScanner(os.Stdin), c)
}

func handleserver(c conn.Conn) {
	defer c.Stop()

	for m := range conn.Messages(c) {
		switch m.T {
		case mes.PingType:
			fmt.Printf("< ping\n")
			conn.Send(c, mes.Pong())
		case mes.JnedType:
			fmt.Printf("< (%d, %v) joined\n", m.Room, m.Name)
		case mes.ExedType:
			fmt.Printf("< (%d, %v) exited\n", m.Room, m.Name)
		case mes.HearType:
			fmt.Printf("< (%d, %v) %v\n", m.Room, m.Name, m.Text)
		case mes.RolsType:
			fmt.Printf("< room list\n%v\n", m.Text)
		case mes.ProbType:
			fmt.Printf("< (error) %v\n", m.Error())
		}
	}
}

func handlescanner(sc *bufio.Scanner, c conn.Conn) {
	defer c.Stop()
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
			if !conn.Send(c, mes.Join(room, name)) {
				goto end
			}
		case "exit":
			if !conn.Send(c, mes.Exit(room)) {
				goto end
			}
		case "talk":
			if len(toks) == 2 {
				fmt.Fprintln(os.Stderr, "! missing text")
				continue
			}
			text := toks[2]
			m := mes.Talk(room, text)
			fmt.Println("the message is", m)
			if !conn.Send(c, m) {
				goto end
			}
		case "ls":
			if !conn.Send(c, mes.Lsro()) {
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

		if conn.Closed(c) {
			return
		}
	}

end:
	if err := sc.Err(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

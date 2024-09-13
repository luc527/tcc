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

	go pc.produceinc(conn)
	go pc.consumeoutc(conn)
	go handleserver(pc)

	respace := regexp.MustCompile(`\s+`)

	sc := bufio.NewScanner(os.Stdin)
loop:
	for sc.Scan() {
		toks := respace.Split(sc.Text(), 3)
		if len(toks) == 0 {
			fmt.Println("! missing command")
			continue
		}
		cmd := toks[0]
		if cmd == "q" {
			break loop
		}
		if len(toks) == 1 {
			fmt.Println("! missing room")
			continue
		}
		sroom := toks[1]
		iroom, err := strconv.ParseUint(sroom, 10, 32)
		if err != nil {
			fmt.Printf("invalid room: %v\n", err)
			continue
		}
		room := uint32(iroom)

		switch cmd {
		case "q":
			break loop
		case "j":
			if len(toks) == 2 {
				fmt.Println("! missing name")
				continue
			}
			name := toks[2]
			m := protomes{t: mjoin, room: room, name: name}
			if !pc.trysend(pc.outc, m) {
				return
			}
		case "e":
			m := protomes{t: mexit, room: room}
			if !pc.trysend(pc.outc, m) {
				return
			}
		case "s":
			if len(toks) == 2 {
				fmt.Println("! missing text")
				continue
			}
			text := toks[2]
			m := protomes{t: msend, room: room, text: text}
			if !pc.trysend(pc.outc, m) {
				return
			}
		}
	}

	if err := sc.Err(); err != nil {
		fmt.Println(err)
	}
}

func handleserver(pc protoconn) {
	for m := range pc.messages() {
		switch m.t {
		case mping:
			pc.trysend(pc.outc, protomes{t: mpong})
		case mjned:
			fmt.Printf("< (%d, %v) joined\n", m.room, m.name)
		case mexed:
			fmt.Printf("< (%d, %v) exited\n", m.room, m.name)
		case mrecv:
			fmt.Printf("< (%d, %v) %v\n", m.room, m.name, m.text)
		case mprob:
			perr, ok := protoerrcode(uint8(m.room))
			s := "invalid code"
			if ok {
				s = perr.Error()
			}
			fmt.Printf("< (error) %v\n", s)
		}
	}
}

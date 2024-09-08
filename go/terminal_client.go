package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"regexp"
	"strconv"
)

var (
	respace = regexp.MustCompile(`\s+`)
)

var (
	clog = log.New(os.Stderr, "<cli> ", 0)
)

func termclient(address string, r io.Reader, w io.Writer) error {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return err
	}
	defer conn.Close()
	clog.Printf("connected to %v, waiting for messages...\n", address)

	go func() {
		for {
			rm := mes{}
			if _, err := rm.ReadFrom(conn); err != nil {
				if _, ok := err.(merror); ok {
					clog.Printf("merror: %v\n", err)
					continue
				} else {
					break
				}
			}
			clog.Printf("message: %v\n", rm)
			if rm.t == mping {
				wm := mes{t: mpong}
				if _, err := wm.WriteTo(conn); err != nil {
					clog.Printf("error writing pong: %v", err)
					break
				}
			}
		}
	}()

	sc := bufio.NewScanner(r)
	for sc.Scan() {
		text := sc.Text()
		toks := respace.Split(text, 2)
		if len(toks) == 0 {
			continue
		}
		t0 := toks[0]
		if t0 == "q" {
			break
		} else if t0 == "!" || t0 == "." { //join or exit
			if len(toks) == 1 {
				fmt.Fprintf(w, "not enough tokens; missing room\n")
				continue
			}
			toks = respace.Split(toks[1], 2)
			room, err := strconv.ParseUint(toks[0], 10, 32)
			if err != nil {
				fmt.Fprintf(w, "invalid message; expected room; atoi err: %v\n", err)
				continue
			}
			var m mes
			if t0 == "!" {
				if len(toks) == 1 {
					fmt.Fprintf(w, "not enough tokens; missing name\n")
					continue
				}
				name := toks[1]
				m = mes{t: mjoin, room: uint32(room), name: name}
			} else {
				m = mes{t: mexit, room: uint32(room)}
			}
			if _, err := m.WriteTo(conn); err != nil {
				clog.Printf("error writing message to client: %v", err)
				continue
			}
		} else if room, err := strconv.ParseUint(t0, 10, 32); err == nil { //send
			if len(toks) == 1 {
				fmt.Fprintf(w, "not enough tokens; missing message text\n")
				continue
			}
			text := toks[1]
			m := mes{t: msend, room: uint32(room), text: text}
			if _, err := m.WriteTo(conn); err != nil {
				clog.Printf("error writing message to server: %v", err)
				continue
			}
		} else {
			fmt.Fprintf(w, "invalid message; atoi err: %v\n", err)
			continue
		}
	}

	if err := sc.Err(); err != nil {
		return err
	}

	return nil
}

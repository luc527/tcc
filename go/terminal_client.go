package main

import (
	"bufio"
	"fmt"
	"io"
	"iter"
	"net"
	"regexp"
	"strconv"
)

var (
	respace = regexp.MustCompile(`\s+`)
)

func termconsumer(w io.Writer, ms iter.Seq[protomes]) {
	for m := range ms {
		switch m.t {
		case mping:
			fmt.Fprintf(w, "< ping\n")
		case mjned:
			fmt.Fprintf(w, "< (%d: %v) joined\n", m.room, m.name)
		case mexed:
			fmt.Fprintf(w, "< (%d: %v) left\n", m.room, m.name)
		case mrecv:
			fmt.Fprintf(w, "< (%d: %v) %s\n", m.room, m.name, m.text)
		default:
			fmt.Fprintf(w, "! invalid message %v\n", m)
		}
	}
	fmt.Fprintf(w, "! connection ended\n")
}

func termclient(address string, r io.Reader, w io.Writer) error {
	rawconn, err := net.Dial("tcp", address)
	if err != nil {
		return err
	}
	defer rawconn.Close()

	conn := serverconn{makeconn(backgroundctx())}

	go conn.writeoutgoing(rawconn)
	go conn.readincoming(rawconn)
	go termconsumer(w, conn.messages())

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
			iroom, err := strconv.ParseUint(toks[0], 10, 32)
			room := uint32(iroom)
			if err != nil {
				fmt.Fprintf(w, "invalid message; expected room; atoi err: %v\n", err)
				continue
			}
			if t0 == "!" {
				if len(toks) == 1 {
					fmt.Fprintf(w, "not enough tokens; missing name\n")
					continue
				}
				name := toks[1]
				if !conn.join(room, name) {
					break
				}
			} else {
				if !conn.exit(room) {
					break
				}
			}
		} else if iroom, err := strconv.ParseUint(t0, 10, 32); err == nil { //send
			room := uint32(iroom)
			if len(toks) == 1 {
				fmt.Fprintf(w, "not enough tokens; missing message text\n")
				continue
			}
			text := toks[1]
			if !conn.send(room, text) {
				break
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

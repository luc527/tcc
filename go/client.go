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
	"strings"
	"sync"
	"time"
)

func runClient(address string) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		log.Fatal(err)
	}

	mu := new(sync.Mutex)
	write := func(m msg) {
		mu.Lock()
		defer mu.Unlock()
		if _, err := m.WriteTo(conn); err != nil {
			log.Fatal(err)
		}
	}

	go func() {
		tick := time.Tick(50 * time.Second)
		for range tick {
			write(msg{t: pingMsg})
		}
	}()

	respace := regexp.MustCompile(`\s+`)

	go cliReadFromConn(conn)

	sc := bufio.NewScanner(os.Stdin)
	for {
		if !sc.Scan() {
			break
		}
		for _, s := range strings.Split(sc.Text(), ";") {
			s = strings.Trim(s, " \n\r\t")
			ss := respace.Split(s, 3)

			if len(ss) == 0 {
				fmt.Printf("< command?\n")
				continue
			}
			cmd, ss := ss[0], ss[1:]

			if len(ss) == 0 {
				fmt.Printf("< topic?\n")
				continue
			}
			topicStr, ss := ss[0], ss[1:]

			topic64, err := strconv.ParseUint(topicStr, 10, 16)
			if err != nil {
				fmt.Printf("< topic: %v\n", err)
				continue
			}
			topic := uint16(topic64)

			var payload string
			if cmd == "pub" {
				if len(ss) == 0 {
					fmt.Printf("< payload?\n")
					continue
				}
				payload, ss = ss[0], ss[1:]
			}

			if len(ss) != 0 {
				fmt.Printf("< excess: %v\n", ss)
				continue
			}

			switch cmd {
			case "sub":
				write(msg{t: subMsg, topic: topic})
			case "unsub":
				write(msg{t: unsubMsg, topic: topic})
			case "pub":
				write(msg{t: pubMsg, topic: topic, payload: payload})
			default:
				fmt.Printf("< unknown command %q\n", cmd)
			}
		}
	}

	if err := sc.Err(); err != nil {
		log.Fatal(err)
	}
}

func cliReadFromConn(r io.Reader) {
	count := 0
	for {
		var m msg
		if _, err := m.ReadFrom(r); err != nil {
			if err != io.EOF {
				log.Fatal(err)
			}
		} else {
			count++
			fmt.Printf("< (%5d) %v\n", count, m)
		}
	}
}

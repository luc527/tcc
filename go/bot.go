package main

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"strings"
	"time"
)

// just testing

func bot0(name string, c serverconn) {
	defer c.cancel()

	var rooms []uint32
	for range 4 {
		rooms = append(rooms, rand.Uint32()%10)
	}

	words := []string{"lorem", "ipsum", "here", "are", "words", "such", "as", "I", "am", "you", "are", "he", "she", "it", "is", "and", "so", "on", "forth", "robot", "human", "game", "agree", "disagree", "dog", "cat", "house", "prison", "window", "door", "lamp", "universal", "contradictory", "algorithm", "stupid", "smart", "strange", "nice", "gross", "all", "thing", "of", "in", "at", "onto", "into", "what"}

	g := newgroup()
	for _, room := range rooms {
		g.run(func() {
			c.join(room, name)
			for range 10 {
				message := make([]string, 2+rand.Intn(20))
				for i := range message {
					message[i] = words[rand.Intn(len(words))]
				}
				c.send(room, strings.Join(message, " "))
				wait := time.Duration(1000+rand.Intn(3000)) * time.Millisecond
				time.Sleep(wait)
			}
			c.exit(room)
		})
	}
	g.wait()
}

func runbots(address string) {
	names := []string{"asdf", "acme", "argh", "uhoh", "botty", "h", "fnarg", "foo", "bar", "baz", "qux", "official"}
	type hist struct {
		name string
		ms   []protomes
	}
	histc := make(chan hist)
	for _, name := range names {
		rawconn, err := net.Dial("tcp", address)
		if err != nil {
			log.Fatal(err)
		}
		conn := serverconn{makeconn(backgroundctx())}
		go conn.writeoutgoing(rawconn)
		go conn.readincoming(rawconn)
		go bot0(name, conn)
		go func() {
			h := hist{name, nil}
			conn.consume(func(m protomes, ok bool) {
				if !ok {
					histc <- h
					return
				}
				h.ms = append(h.ms, m)
			})
		}()
	}
	for range names {
		hist := <-histc
		fmt.Printf("\n==== History for [%v] ====\n", hist.name)
		for _, m := range hist.ms {
			fmt.Println(m)
		}
	}
}

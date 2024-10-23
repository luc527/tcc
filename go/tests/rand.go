package tests

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"maps"
	"math/rand/v2"
	"net"
	"slices"
	"tccgo/chk"
	"tccgo/conn"
	"tccgo/mes"
	"time"
)

var (
	vowels     = []byte("aeiou")
	consonants = []byte("bcdfghjklmnpqrstvwxyz")
)

type randspec struct {
	nstep    int
	nroom    int
	nconn    int
	talkprob float64
}

type randconn struct {
	ctx    context.Context
	cancel context.CancelFunc
	rooms  map[uint32]string
	conn   conn.Conn
}

func randomVowel() byte {
	r := rand.Int() % len(vowels)
	return vowels[r]
}

func randomConsonant() byte {
	r := rand.Int() % len(consonants)
	return consonants[r]
}

func writeRandomWord(bb *bytes.Buffer, parts int) {
	prev := -1
	for range parts {
		p := -1
		for p == prev {
			p = rand.Int() % 3
		}
		prev = p
		switch p {
		case 0:
			bb.WriteByte(randomVowel())
		case 1:
			bb.WriteByte(randomConsonant())
			bb.WriteByte(randomVowel())
		case 2:
			bb.WriteByte(randomConsonant())
			bb.WriteByte(randomVowel())
			bb.WriteByte(randomConsonant())
		}
	}
}

func writeRandomWords(bb *bytes.Buffer, n int) {
	sep := ""
	for range n {
		bb.WriteString(sep)
		writeRandomWord(bb, max(1, rand.Int()%8))
		sep = " "
	}
}

func runRandom(rspec randspec, address string) {
	check := chk.NewChecker()
	conns := make([]randconn, rspec.nconn)

	for i := range conns {
		raw, err := net.Dial("tcp", address)
		if err != nil {
			log.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		rconn := randconn{
			ctx:    ctx,
			cancel: cancel,
			rooms:  make(map[uint32]string),
			conn:   conn.New(ctx, cancel),
		}
		rconn.conn.Start(raw)
		go func() {
			for m := range conn.Messages(rconn.conn) {
				fmt.Printf("(r%d) %v\n", i, m)
			}
		}()
		conns[i] = rconn
		fmt.Printf("conn %3d started\n", i)
	}

	tick := time.Tick(48 * time.Millisecond)
	bb := new(bytes.Buffer)

	randtext := func() string {
		bb.Reset()
		minw, maxw := 3, 10
		writeRandomWords(bb, minw+rand.IntN(maxw-minw))
		return bb.String()
	}

	for step := range rspec.nstep {
		var m mes.Message
		rconni := rand.IntN(len(conns))
		rconn := conns[rconni]

		randroom := func() uint32 {
			optroom := slices.Collect(maps.Keys(rconn.rooms))
			room := optroom[rand.IntN(len(optroom))]
			return uint32(room)
		}

		randjoin := func() uint32 {
			optjoin := make([]uint32, 0)
			for room := range uint32(rspec.nroom) {
				if _, ok := rconn.rooms[room]; !ok {
					optjoin = append(optjoin, room)
				}
			}
			room := optjoin[rand.IntN(len(optjoin))]
			return uint32(room)
		}

		joinname := func() string {
			return string([]rune{rune('a' + rconni)})
		}

		cantalkorexit := len(rconn.rooms) > 0
		canjoin := len(rconn.rooms) < rspec.nroom

		if cantalkorexit {
			if rand.Float64() < rspec.talkprob {
				m = mes.Talk(randroom(), randtext())
			} else if !canjoin || rand.Float64() < 0.5 {
				room := randroom()
				m = mes.Exit(room)
				delete(rconn.rooms, room)
			} else {
				room, name := randjoin(), joinname()
				m = mes.Join(room, name)
				rconn.rooms[room] = name
			}
		} else if canjoin {
			room, name := randjoin(), joinname()
			m = mes.Join(room, name)
			rconn.rooms[room] = name
		}

		check.Accept(rconni, m)

		<-tick
		fmt.Printf("step %3d\n(r%d) SENDING %v\n", step, rconni, m)
		conn.Send(rconn.conn, m)
	}

	time.Sleep(200 * time.Millisecond)
	for _, rconn := range conns {
		rconn.conn.Stop()
	}
	time.Sleep(200 * time.Millisecond)

	res := check.Results()
	fmt.Println("missing:")
	for m := range res.Missing() {
		fmt.Println(m)
	}
}

func Randmain(address string) {
	rspec := randspec{
		nstep:    200,
		nroom:    10,
		nconn:    6,
		talkprob: 0.8,
	}
	runRandom(rspec, address)
}

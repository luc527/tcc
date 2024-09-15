package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strconv"
	"time"
)

type botspec struct {
	dur time.Duration
	nw  int
	nm  int
}

type army struct {
	cancel context.CancelFunc
	bots   []bot
}

type joinspec struct {
	room uint32
	name string
}

type bot struct {
	spec    botspec
	connser sender[protomes]
	join    chan joinspec
	exit    chan uint32
}

var (
	namedbotspecs = map[string]botspec{
		"tinyslow": botmustparse("5s 3 -1"),
	}
	wordpool = []string{"hello", "goodbye", "ok", "yeah", "nah", "true", "false", "is", "it", "the", "do", "don't", "they", "you", "them", "your", "me", "I", "mine", "dog", "cat", "duck", "robot", "squid", "tiger", "lion", "snake", "truth", "lie"}
)

func conmain(address string) {
	sc := bufio.NewScanner(os.Stdin)

	armies := make(map[string]army)

	defer func() {
		for _, army := range armies {
			army.cancel()
		}
	}()

	for sc.Scan() {
		toks := respace.Split(sc.Text(), -1)
		cmd, args := toks[0], toks[1:]

		nextarg := func() (string, bool) {
			if len(args) == 0 {
				return "", false
			}
			defer func() {
				args = args[1:]
			}()
			return args[0], true
		}

		if cmd == "quit" {
			fmt.Println("< bye")
			break
		}

		if cmd == "spawn" {
			armyname, ok := nextarg()
			if !ok {
				fmt.Printf("! missing army name\n")
				continue
			}
			sarmysize, ok := nextarg()
			if !ok {
				fmt.Printf("! missing army size\n")
				continue
			}
			uarmysize, err := strconv.ParseUint(sarmysize, 10, 32)
			if err != nil {
				fmt.Printf("! invalid army size: %v\n", err)
				continue
			}
			armysize := uint(uarmysize)
			aspec := args
			if !ok {
				fmt.Printf("! missing spec\n")
				continue
			}
			var spec botspec
			switch len(aspec) {
			case 1:
				spec, ok = namedbotspecs[aspec[0]]
				if !ok {
					fmt.Printf("! spec named %q not found\n", aspec[0])
					continue
				}
			case 3:
				spec, err = botparsea(aspec)
				if err != nil {
					fmt.Printf("! %v\n", err)
					continue
				}
			default:
				fmt.Printf("! invalid spec length %d\n", len(aspec))
				continue
			}
			army, err := startarmy(spec, address, armysize)
			if err != nil {
				fmt.Printf("! failed to start the bot army: %v\n", err)
				continue
			}
			fmt.Printf("< army %q spawned\n", armyname)
			armies[armyname] = army
		} else if cmd == "kill" {
			armyname, ok := nextarg()
			if !ok {
				fmt.Printf("! missing army name\n")
				continue
			}
			army, ok := armies[armyname]
			if !ok {
				fmt.Printf("! army not found\n")
				continue
			}
			army.cancel()
			delete(armies, armyname)
		} else if cmd == "join" || cmd == "exit" {
			armyname, ok := nextarg()
			if !ok {
				fmt.Printf("! missing army name\n")
				continue
			}
			army, ok := armies[armyname]
			if !ok {
				fmt.Printf("! army not found\n")
				continue
			}
			sroom, ok := nextarg()
			if !ok {
				fmt.Printf("! missing room\n")
				continue
			}
			uroom, err := strconv.ParseUint(sroom, 10, 32)
			if err != nil {
				fmt.Printf("! invalid room: %v\n", err)
				continue
			}
			room := uint32(uroom)
			if cmd == "join" {
				name, ok := nextarg()
				if !ok {
					fmt.Printf("! missing user name\n")
					continue
				}
				army.join(room, name)
				fmt.Printf("< army %q joined room %d with names starting with %q\n", armyname, room, name)
			} else {
				army.exit(room)
				fmt.Printf("< army %q exited room %d\n", armyname, room)
			}
		}

	}

	if err := sc.Err(); err != nil {
		fmt.Printf("! scanner: %v", err)
	}
}

func (bs botspec) sendmessages(s sender[string]) {
	defer s.close()

	if bs.nm == 0 {
		return
	}
	if bs.nw == 0 {
		return
	}

	minw := bs.nw
	maxw := bs.nw
	if bs.nw < 0 {
		third := -bs.nw / 3
		minw = -bs.nw - third
		maxw = -bs.nw + third
	}

	// use same buffer for writing messages
	// ensuring constant memory use
	maxlength := 0
	for _, word := range wordpool {
		maxlength = max(len(word), maxlength)
	}
	bbuf := new(bytes.Buffer)
	bbuf.Grow(maxw * (maxlength + 1))
	// + 1 to account for the spaces between words

	mindur := int64(bs.dur)
	maxdur := int64(bs.dur)
	if bs.dur < 0 {
		third := int64(-bs.dur) / 3
		mindur = int64(-bs.dur) - third
		maxdur = int64(-bs.dur) + third
	}
	sent := 0

	for {
		bbuf.Reset()
		n := minw
		if d := maxw - minw; d > 0 {
			n += rand.Intn(d)
		}
		sep := ""
		for range n {
			word := wordpool[rand.Intn(len(wordpool))]
			bbuf.WriteString(sep)
			bbuf.WriteString(word)
			sep = " "
		}
		message := bbuf.String()

		select {
		case <-s.done:
			return
		default:
			s.send(message)
		}

		sent++
		if bs.nm > 0 && sent >= bs.nm {
			break
		}

		dur := mindur
		if d := maxdur - mindur; d > 0 {
			dur += rand.Int63() % d
		}
		time.Sleep(time.Duration(dur))
	}
}

func botmustparse(s string) botspec {
	b, err := botparses(s)
	if err != nil {
		panic(err)
	}
	return b
}

func botparses(s string) (botspec, error) {
	return botparsea(respace.Split(s, -1))
}

func botparsea(args []string) (bs botspec, rerr error) {
	if len(args) != 3 {
		rerr = fmt.Errorf("botspec: expected 3 args, received %d", len(args))
		return
	}
	dur, err := time.ParseDuration(args[0])
	if err != nil {
		rerr = fmt.Errorf("botspec: invalid duration: %w", err)
		return
	}
	nw, err := strconv.Atoi(args[1])
	if err != nil {
		rerr = fmt.Errorf("botspec: invalid number of words: %w", err)
		return
	}
	nm, err := strconv.Atoi(args[2])
	if err != nil {
		rerr = fmt.Errorf("botspec: invalid number of messages: %w", err)
		return
	}
	bs = botspec{dur, nw, nm}
	return
}

func startarmy(spec botspec, address string, size uint) (a army, e error) {
	ctx, cancel := context.WithCancel(context.Background())

	bots := make([]bot, size)
	for i := range bots {
		rawconn, err := net.Dial("tcp", address)
		if err != nil {
			cancel()
			e = err
			return
		}

		pc := makeconn(rawconn)
		go pc.producein()
		go pc.consumeout()

		connser := pc.sender()
		context.AfterFunc(ctx, connser.close)

		b := makebot(spec, connser)
		bots[i] = b

		go b.handlemessages(pc.receiver())
		go b.main()
	}

	a = army{cancel, bots}
	return
}

func (a army) join(room uint32, prefix string) {
	for i, b := range a.bots {
		name := fmt.Sprintf("%s%d", prefix, i)
		select {
		case b.join <- joinspec{room, name}:
		case <-b.connser.done:
		}
	}
}

func (a army) exit(room uint32) {
	for _, b := range a.bots {
		select {
		case b.exit <- room:
		case <-b.connser.done:
		}
	}
}

func makebot(spec botspec, s sender[protomes]) bot {
	join := make(chan joinspec)
	exit := make(chan uint32)
	return bot{spec, s, join, exit}
}

func (b bot) handlemessages(r receiver[protomes]) {
	for {
		m, ok := r.receive()
		if !ok {
			break
		}
		if m.t == mping {
			b.connser.send(protomes{t: mpong})
		}
	}
}

func (b bot) main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rooms := make(map[uint32]context.CancelFunc)
	endedc := make(chan uint32)

	for {
		select {
		case join := <-b.join:
			if _, joined := rooms[join.room]; !joined {
				m := protomes{t: mjoin, name: join.name, room: join.room}
				b.connser.send(m)

				roomctx, cancelroom := context.WithCancel(ctx)
				rooms[join.room] = cancelroom

				f := freer[uint32]{
					done: b.connser.done,
					c:    endedc,
					id:   join.room,
				}

				textc := make(chan string)
				textser, textrer := senderreceiver(roomctx.Done(), textc, cancelroom)

				go b.spec.sendmessages(textser)
				go roombotmain(join.room, b.connser, textrer, f)
			}
		case room := <-b.exit:
			if cancelroom, joined := rooms[room]; joined {
				cancelroom()
			}
		case room := <-endedc:
			delete(rooms, room)
			m := protomes{t: mexit, room: room}
			b.connser.send(m)
		case <-b.connser.done:
			return
		}
	}
}

func roombotmain(room uint32, s sender[protomes], r receiver[string], f freer[uint32]) {
	defer f.free()
	// TODO: test delay
	delay := time.Duration(rand.Uint64() % uint64(5*time.Second))
	time.Sleep(delay)
	for {
		text, ok := r.receive()
		if !ok {
			break
		}
		m := protomes{t: msend, room: room, text: text}
		s.send(m)
	}
}

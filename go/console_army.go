package main

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"sync/atomic"
	"time"
)

// TODO: after killing army, connend ends up unfulfilled for most bots
// probably something to do with the other bots also being disconnected in the +- same moment

var (
	namedbotspecs = map[string]botspec{ /*TODO*/ }
	wordpool      = []string{"hello", "goodbye", "ok", "yeah", "nah", "true", "false", "is", "it", "the", "do", "don't", "they", "you", "them", "your", "me", "I", "mine", "dog", "cat", "duck", "robot", "squid", "tiger", "lion", "snake", "truth", "lie"}
	nextbotid     atomic.Uint64
)

type botspec struct {
	dur time.Duration
	nw  int
	nm  int
}

func (bs botspec) messages(done <-chan zero) <-chan string {
	c := make(chan string)
	go bs.sendmessages(done, c)
	return c
}

func (bs botspec) sendmessages(done <-chan zero, c chan<- string) {
	defer close(c)
	// ^ so it's very important for the receiver do to "text, ok := <- texts; if !ok { break }"
	// otherwise, the channel will repeatedly yield the zero value "" (that's what receiving on a closed channel does)
	// and the bot will repeatedly send a {talk, ""} message
	// with absolutely NO delay between the messages. it'll really go at it.

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
		case <-done:
			return
		case c <- message:
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

type army struct {
	ctx    context.Context
	cancel context.CancelFunc
	bots   []bot
}

// too many args..
func startarmy(ctx context.Context, cancel context.CancelFunc, size uint, spec botspec, name string, address string, f func(string) middleware, startf func(string), endf func(string)) (a army, e error) {
	a = army{
		ctx:    ctx,
		cancel: cancel,
		bots:   make([]bot, size),
	}
	for i := range a.bots {
		rawconn, err := net.Dial("tcp", address)
		if err != nil {
			a.cancel()
			e = err
			return
		}
		// close the bots gradually
		wc := waitcloser{
			c: rawconn,
			d: time.Duration(128 * int64(i) * int64(time.Millisecond)),
		}
		connName := fmt.Sprintf("%s%d", name, nextbotid.Add(1))
		cctx, ccancel := context.WithCancel(ctx)

		logid := fmt.Sprintf("army/%s", connName)
		if startf != nil {
			startf(logid)
		}
		if endf != nil {
			context.AfterFunc(cctx, func() { endf(logid) })
		}

		pc := makeconn(cctx, ccancel).
			start(rawconn, wc).
			withmiddleware(f(logid))
		b := bot{
			pc:   pc,
			spec: spec,
			join: make(chan joinspec),
			exit: make(chan uint32),
		}
		a.bots[i] = b
		go b.handleping()
		go b.main()
	}
	return
}

func (a army) join(room uint32, prefix string) {
	// throttle - don't all join at once
	ticker := time.NewTicker(128 * time.Millisecond)
	defer ticker.Stop()
	for i, b := range a.bots {
		name := fmt.Sprintf("%s%d", prefix, i)
		js := joinspec{room, name}
		trysend(b.join, js, b.pc.ctx.Done())
		prf("< bot [%d] joined\n", i)
		<-ticker.C
	}
}

func (a army) exit(room uint32) {
	// throttle - don't all exit at once
	ticker := time.NewTicker(128 * time.Millisecond)
	defer ticker.Stop()
	for i, b := range a.bots {
		trysend(b.exit, room, b.pc.ctx.Done())
		prf("< bot [%d] exited\n", i)
		<-ticker.C
	}
}

type bot struct {
	pc   protoconn
	spec botspec
	join chan joinspec
	exit chan uint32
}

func (b bot) handleping() {
	goinc()
	defer godec()
	defer b.pc.cancel()
	for {
		select {
		case <-b.pc.ctx.Done():
			return
		case m := <-b.pc.in:
			if m.t == mping {
				if !b.pc.send(protomes{t: mpong}) {
					return
				}
			}
		}
	}
}

func (b bot) main() {
	goinc()
	defer godec()
	defer b.pc.cancel()

	rooms := make(map[uint32]context.CancelFunc)
	ended := make(chan uint32)

	for {
		select {
		case <-b.pc.ctx.Done():
			return
		case room := <-b.exit:
			if cancel, joined := rooms[room]; joined {
				cancel()
			}
		case room := <-ended:
			delete(rooms, room)
			m := protomes{t: mexit, room: room}
			if !b.pc.send(m) {
				return
			}
		case join := <-b.join:
			room := join.room
			if _, joined := rooms[room]; joined {
				continue
			}
			m := protomes{t: mjoin, name: join.name, room: join.room}
			if !b.pc.send(m) {
				return
			}

			ctx, cancel := context.WithCancel(b.pc.ctx)
			context.AfterFunc(ctx, func() {
				trysend(ended, room, b.pc.ctx.Done())
			})
			rooms[room] = cancel

			texts := b.spec.messages(ctx.Done())
			rb := roombot{
				ctx:    ctx,
				cancel: cancel,
				texts:  texts,
				out:    b.pc.out, // TODO: is this ok?
			}
			go rb.main(room)
		}
	}
}

type roombot struct {
	ctx    context.Context
	cancel context.CancelFunc
	texts  <-chan string
	out    chan<- protomes
}

// TODO: send output when all bots from a room finished
// TODO: periodically output # of bots from each army running on each room

func (rb roombot) main(room uint32) {
	goinc()
	defer godec()
	defer rb.cancel()

	delay := time.Duration(rand.Uint64() % uint64(5*time.Second))
	time.Sleep(delay)

	for {
		select {
		case <-rb.ctx.Done():
			return
		case text, ok := <-rb.texts:
			if !ok {
				return
			}
			m := protomes{t: mtalk, room: room, text: text}
			if !trysend(rb.out, m, rb.ctx.Done()) {
				return
			}
		}
	}
}

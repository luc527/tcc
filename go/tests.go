package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// NOTE: all this code is not particularly well written

type testproc struct {
	ctx        context.Context
	cancel     context.CancelFunc
	createTime time.Time
	id         int
}

type topicproc struct {
	testproc
	interval  time.Duration
	topic     uint16
	nclients  int
	msglen    int
	sendCount *atomic.Int32
	// TODO: bottleneck? too much contention! instead, multiple counters, synchronized with a global counter just every once in a while. maybe even per-conn: only synchronize every 100 recvs
	recvCount *atomic.Int32
}

type procs struct {
	mu    sync.Mutex
	topic []*topicproc
}

func (p *procs) makeTopicproc(topic uint16, nclients int, msglen int, interval time.Duration) *topicproc {
	// TODO: msglen -> msgfun func() string?
	p.mu.Lock()
	defer p.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	createTime := time.Now()
	id := len(p.topic)
	tp := topicproc{
		testproc:  testproc{ctx, cancel, createTime, id},
		topic:     topic,
		nclients:  nclients,
		msglen:    msglen,
		interval:  interval,
		sendCount: new(atomic.Int32),
		recvCount: new(atomic.Int32),
	}
	tpp := &tp
	p.topic = append(p.topic, tpp)
	return tpp
}

func (p *procs) ls() {
	fmt.Printf("\ntopic procs\n")
	for _, tp := range p.topic {
		fmt.Printf(
			"%3d  %s created  %7s interval  %5d topic  %5d clients  %3d msglen  %7d send  %7d recv \n",
			tp.id,
			tp.createTime.Format(time.TimeOnly),
			tp.interval.String(),
			tp.topic,
			tp.nclients,
			tp.msglen,
			tp.sendCount.Load(),
			tp.recvCount.Load(),
		)
	}
}

var (
	respace = regexp.MustCompile(`\s+`)
	rekv    = regexp.MustCompile(`(\w+)=([\w\d]+)`)
)

func nextarg(args []string, name string) (ok bool, arg string, rest []string) {
	if len(args) == 0 {
		fmt.Printf("%v?\n", name)
		return false, "", nil
	}
	ok = true
	arg = args[0]
	rest = args[1:]
	return
}

func runTests(address string) {
	procs := &procs{
		mu:    sync.Mutex{},
		topic: nil,
	}

	var prev []string

	scanner := bufio.NewScanner(os.Stdin)
scannerLoop:
	for scanner.Scan() {
		txt := strings.Trim(scanner.Text(), " \t\n\r")
		args := respace.Split(txt, -1)

		if strings.Contains(txt, "XXX") {
			fmt.Printf("ignored\n")
			continue
		}

	handleArgs:
		ok, cmd, args := nextarg(args, "command")
		if !ok {
			break
		}

		// TODO: "again" command not working
		// TODO: "kill" command
		switch cmd {
		case "again":
			args = prev
			goto handleArgs
		case "quit":
			break scannerLoop
		case "ls":
			procs.ls()
		case "load-topic":
			var topic uint16
			itopic := -1
			nclients := -1
			msglen := 12
			interval := 5 * time.Second

			for _, arg := range args {
				groups := rekv.FindStringSubmatch(arg)
				if groups == nil {
					fmt.Printf("expected key-value, as in topic=120")
					break
				}
				_, k, v := groups[0], groups[1], groups[2]
				// NOTE: every `case` is pretty much the same, except for "interval"
				switch k {
				case "topic":
					v, err := strconv.ParseUint(v, 10, 16)
					if err != nil {
						fmt.Printf("topic: %v\n", err)
						continue
					}
					itopic = int(v)
					topic = uint16(itopic)
				case "nclients":
					v, err := strconv.ParseUint(v, 10, 31)
					if err != nil {
						fmt.Printf("nclients: %v\n", err)
						continue
					}
					nclients = int(v)
				case "msglen":
					v, err := strconv.ParseUint(v, 10, 31)
					if err != nil {
						fmt.Printf("msglen: %v\n", err)
						continue
					}
					msglen = int(v)
				case "interval":
					v, err := time.ParseDuration(v)
					if err != nil {
						fmt.Printf("interval: %v\n", err)
						continue
					}
					interval = v
				}
			}

			if nclients == -1 {
				fmt.Printf("nclients?\n")
				break
			}

			if topic == 0 {
				fmt.Printf("topic?\n")
				break
			}

			tproc := procs.makeTopicproc(topic, nclients, msglen, interval)

			nfailed := 0
			for range nclients {
				conn, err := net.Dial("tcp", address)
				if err != nil {
					fmt.Printf("tcp dial: %v\n", err)
					nfailed++
					continue
				}

				ctx, cancel := context.WithCancel(tproc.ctx)
				context.AfterFunc(ctx, func() {
					conn.Close()
				})
				go runTopicClient(ctx, cancel, conn, topic, interval, msglen, tproc.sendCount, tproc.recvCount)
			}

			if nfailed > 0 {
				fmt.Printf("! started only %v out of %v clients\n", nclients-nfailed, nclients)
			}
		}

		if cmd != "again" {
			prev = args
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("scanner: %v", err)
	}
}

func runTopicClient(
	ctx context.Context,
	cancel context.CancelFunc,
	conn net.Conn,
	topic uint16,
	interval time.Duration,
	msglen int,
	sendCount *atomic.Int32,
	recvCount *atomic.Int32,
) {
	defer cancel()

	subm := msg{t: subMsg, topic: topic, b: true}
	if _, err := subm.WriteTo(conn); err != nil {
		fmt.Printf("failed to subscribe: %v\n", err)
		return
	}

	// go io.Copy(io.Discard, conn)
	go func() {
		for {
			m := msg{}
			if _, err := m.ReadFrom(conn); err != nil {
				fmt.Printf("failed to read message: %v\n", err)
				break
			}
			recvCount.Add(1)
		}
	}()

	// NOTE: not necessary to send pings, since we are already sending regular messages
	// at an interval smaller than the server inactivity timeout

	tick := time.Tick(interval)
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick:
			randbytes := make([]byte, msglen/2)
			if _, err := rand.Read(randbytes); err != nil {
				fmt.Printf("rand read failed: %v\n", err)
				return
			}
			payload := make([]byte, 0, msglen+1) // + 1 just in case my math is wrong
			for _, b := range randbytes {
				x := (b & 0x1F)
				y := (b & 0xE0) >> 5
				// look at the ascii table and this will make sense {
				x = '[' + x
				if x == '\\' {
					x = '/'
				}
				payload = append(payload, x)
				payload = append(payload, 'a'+y)
				// }
			}

			m := msg{t: pubMsg, topic: topic, payload: string(payload)}
			if _, err := m.WriteTo(conn); err != nil {
				fmt.Printf("failed to send message: %v\n", err)
				return
			}
			sendCount.Add(1)
		}
	}
}

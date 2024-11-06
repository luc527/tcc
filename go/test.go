package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func nextarg(args []string, name string) (bool, string, []string) {
	if len(args) == 0 {
		fmt.Printf("%v?\n", name)
		return false, "", nil
	}
	return true, args[0], args[1:]
}

var (
	respace = regexp.MustCompile(`\s+`)
)

type proc struct {
	id     int
	ctx    context.Context
	cancel context.CancelFunc
	nconn  int
	topic  uint16
	count  *atomic.Int32
}

type subproc struct {
	proc
}

type pubproc struct {
	proc
	msglen   int
	interval time.Duration
}

type testConsole struct {
	mu sync.Mutex

	address           string
	sendSyncThreshold int
	recvSyncThreshold int

	subNextid int
	subs      map[int]subproc

	pubNextid int
	pubs      map[int]pubproc
}

func (tc *testConsole) spawnSub(nconn int, topic uint16) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	count := &atomic.Int32{}
	for range nconn {
		conn, err := net.Dial("tcp", tc.address)
		if err != nil {
			cancel()
			fmt.Printf("failed to connect: %v\n", err)
			return
		}
		connctx, conncancel := context.WithCancel(ctx)
		go runSubConn(connctx, conncancel, conn, topic, count, tc.recvSyncThreshold)
	}
	id := tc.subNextid
	subproc := subproc{
		proc{
			id:     id,
			ctx:    ctx,
			cancel: cancel,
			nconn:  nconn,
			topic:  topic,
			count:  count,
		},
	}
	tc.subNextid++
	tc.subs[id] = subproc
}

func runSubConn(ctx context.Context, cancel context.CancelFunc, conn net.Conn, topic uint16, count *atomic.Int32, syncThreshold int) {
	defer cancel()

	go ping(conn, ctx)

	m := msg{t: subMsg, topic: topic, b: true}
	if _, err := m.WriteTo(conn); err != nil {
		fmt.Printf("failed to subscribe: %v\n", err)
		return
	}

	local := 0
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		m := msg{}
		if _, err := m.ReadFrom(conn); err != nil {
			fmt.Printf("failed to receive: %v\n", err)
			return
		}
		local++
		if local == syncThreshold {
			count.Add(int32(local))
			local = 0
		}
	}
}

func ping(conn net.Conn, ctx context.Context) {
	tick := time.Tick(50 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick:
			m := msg{t: pingMsg}
			if _, err := m.WriteTo(conn); err != nil {
				fmt.Printf("failed to ping: %v\n", err)
				return
			}
		}
	}
}

func (tc *testConsole) spawnPub(nconn int, topic uint16, msglen int, interval time.Duration) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	count := &atomic.Int32{}
	for range nconn {
		conn, err := net.Dial("tcp", tc.address)
		if err != nil {
			cancel()
			fmt.Printf("failed to connect: %v\n", err)
			return
		}
		connctx, conncancel := context.WithCancel(ctx)
		go runPubConn(connctx, conncancel, conn, topic, msglen, interval, count, tc.sendSyncThreshold)
	}
	id := tc.pubNextid
	pubproc := pubproc{
		proc: proc{
			id:     id,
			ctx:    ctx,
			cancel: cancel,
			nconn:  nconn,
			topic:  topic,
			count:  count,
		},
		msglen:   msglen,
		interval: interval,
	}
	tc.pubNextid++
	tc.pubs[id] = pubproc
}

func runPubConn(ctx context.Context, cancel context.CancelFunc, conn net.Conn, topic uint16, msglen int, interval time.Duration, count *atomic.Int32, syncThreshold int) {
	defer cancel()

	local := 0
	tick := time.Tick(interval)
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick:
			m := msg{t: pubMsg, topic: topic, payload: makePayload(msglen)}
			if _, err := m.WriteTo(conn); err != nil {
				fmt.Printf("failed to send: %v\n", err)
				return
			}
			local++
			if local == syncThreshold {
				count.Add(int32(local))
				local = 0
			}
		}
	}
}

func makePayload(n int) string {
	bs := make([]byte, n)
	if _, err := rand.Read(bs); err != nil {
		fmt.Printf("failed to make payload: %v\n", err)
		return ""
	}
	for i := range bs {
		bs[i] = 'a' + bs[i]%26
	}
	return string(bs)
}

func (tc *testConsole) killPub(id int) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if pubproc, ok := tc.pubs[id]; ok {
		pubproc.cancel()
		delete(tc.pubs, id)
	}
}

func (tc *testConsole) killSub(id int) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if subproc, ok := tc.subs[id]; ok {
		subproc.cancel()
		delete(tc.subs, id)
	}
}

func (tc *testConsole) ls() {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if len(tc.subs) > 0 {
		fmt.Println("subprocs")
		for _, sp := range tc.subs {
			fmt.Printf("%3d  %3d nconn  %5d topic  %11d count\n", sp.id, sp.nconn, sp.topic, sp.count.Load())
		}
	}

	if len(tc.pubs) > 0 {
		fmt.Println("pubprocs")
		for _, pp := range tc.pubs {
			fmt.Printf("%3d  %3d nconn  %5d topic  %11d count  %3d msglen  %9s interval\n", pp.id, pp.nconn, pp.topic, pp.count.Load(), pp.msglen, pp.interval.String())
		}
	}
}

func newTestConsole(address string, recvSyncThreshold int, sendSyncThreshold int) *testConsole {
	return &testConsole{
		mu:                sync.Mutex{},
		address:           address,
		recvSyncThreshold: recvSyncThreshold,
		sendSyncThreshold: sendSyncThreshold,
		subNextid:         1,
		subs:              make(map[int]subproc),
		pubNextid:         1,
		pubs:              make(map[int]pubproc),
	}
}

func runTestConsole(address string, recvSyncThreshold int, sendSyncThreshold int) {
	tc := newTestConsole(address, recvSyncThreshold, sendSyncThreshold)

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		text := strings.Trim(scanner.Text(), " \t\n\r")
		args := respace.Split(text, -1)

		ok, cmd, args := nextarg(args, "command")
		if !ok {
			continue
		}

		if cmd == "quit" {
			fmt.Println("goodbye")
			break
		}

		switch cmd {
		case "spawn":
			ok, kind, args := nextarg(args, "kind")
			if !ok {
				continue
			}
			ok, nconn_, args := nextarg(args, "number of connections?")
			if !ok {
				continue
			}
			ok, topic_, args := nextarg(args, "topic")
			if !ok {
				continue
			}
			nconn, err := strconv.ParseInt(nconn_, 10, 32)
			if err != nil {
				fmt.Printf("invalid nconn: %v\n", err)
				continue
			}
			topic, err := strconv.ParseUint(topic_, 10, 16)
			if err != nil {
				fmt.Printf("invalid topic: %v\n", err)
			}
			switch kind {
			case "pub":
				ok, msglen_, args := nextarg(args, "msglen")
				if !ok {
					continue
				}
				ok, interval_, args := nextarg(args, "interval")
				if !ok {
					continue
				}
				_ = args
				msglen, err := strconv.ParseInt(msglen_, 10, 32)
				if err != nil {
					fmt.Printf("invalid msglen: %v\n", err)
					continue
				}
				interval, err := time.ParseDuration(interval_)
				if err != nil {
					fmt.Printf("invalid interval: %v\n", err)
					continue
				}
				tc.spawnPub(int(nconn), uint16(topic), int(msglen), interval)
			case "sub":
				go tc.spawnSub(int(nconn), uint16(topic))
			default:
				fmt.Printf("unknown kind %q\n", kind)
			}
		case "kill":
			ok, kind, args := nextarg(args, "kind")
			if !ok {
				continue
			}
			ok, id_, args := nextarg(args, "id")
			if !ok {
				continue
			}
			id, err := strconv.ParseInt(id_, 10, 32)
			if err != nil {
				fmt.Printf("invalid id: %v\n", err)
				continue
			}
			_ = args
			switch kind {
			case "pub":
				tc.killPub(int(id))
			case "sub":
				tc.killSub(int(id))
			case "default":
				fmt.Printf("unknown process kind %q\n", kind)
			}
		case "ls":
			tc.ls()
		default:
			fmt.Printf("unknown command %q\n", cmd)
		}

		fmt.Println(time.Now())
	}
}

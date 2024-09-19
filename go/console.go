package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

var l logger

type joinspec struct {
	room uint32
	name string
}

type talkspec struct {
	room uint32
	text string
}

type arglist []string

func (a *arglist) next() (string, bool) {
	if len(*a) == 0 {
		return "", false
	}
	defer func() {
		*a = (*a)[1:]
	}()
	return (*a)[0], true
}

func (a *arglist) rest() []string {
	return (*a)[:]
}

type console struct {
	armies  map[string]army
	clients map[string]conclient
}

func conmain(address string, logname string) {
	var file io.Writer

	if len(logname) > 0 {
		path := fmt.Sprintf("./logs/log_%s_%s.csv", logname, time.Now().Format("20060102_1504"))
		var err error
		file, err = os.Create(path)
		if err != nil {
			log.Printf("failed to open %s", path)
			return
		}
	} else {
		file = io.Discard
	}

	con := console{
		armies:  make(map[string]army),
		clients: make(map[string]conclient),
	}

	l = makelogger(csv.NewWriter(file))
	go l.main()

	sc := bufio.NewScanner(os.Stdin)

	defer func() {
		for _, army := range con.armies {
			army.cancel()
		}
	}()

	for sc.Scan() {
		toks := respace.Split(sc.Text(), -1)
		if toks[0] == "quit" {
			fmt.Fprintln(os.Stderr, "< bye")
			break
		}
		if len(toks) < 2 {
			fmt.Fprintln(os.Stderr, "! <domain> <command> [<args>]")
			continue
		}
		domain, cmd, args := toks[0], toks[1], toks[2:]
		argl0 := arglist(args)
		argl := &argl0

		// TODO: since this may take stdin from a file
		// maybe every loop should have a little bit of sleep, like 10ms
		// just to avoid overloading the server
		// maybe, idk

		switch domain {
		case "meta":
			con.handlemeta(cmd, argl)
		case "army":
			con.handlearmy(address, cmd, argl)
		case "cli":
			con.handleclient(address, cmd, argl)
		default:
			fmt.Fprintf(os.Stderr, "! unknown domain %q\n", domain)
		}

	}

	if err := sc.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "! scanner: %v\n", err)
	}
}

func (c console) handlemeta(cmd string, argl *arglist) {
	if cmd == "sleep" {
		sleeps, ok := argl.next()
		if !ok {
			fmt.Fprintf(os.Stderr, "! missing sleep duration\n")
			return
		}
		sleep, err := time.ParseDuration(sleeps)
		if err != nil {
			fmt.Fprintf(os.Stderr, "! invalid sleep duration: %v\n", err)
			return
		}
		time.Sleep(sleep)
	}
	fmt.Fprintf(os.Stderr, "unknown ctl command %q\n", cmd)
}

func (c console) handlearmy(address string, cmd string, argl *arglist) {
	if cmd == "spawn" {
		armyname, ok := argl.next()
		if !ok {
			fmt.Fprintf(os.Stderr, "! missing army name\n")
			return
		}
		sarmysize, ok := argl.next()
		if !ok {
			fmt.Fprintf(os.Stderr, "! missing army size\n")
			return
		}
		uarmysize, err := strconv.ParseUint(sarmysize, 10, 32)
		if err != nil {
			fmt.Fprintf(os.Stderr, "! invalid army size: %v\n", err)
			return
		}
		armysize := uint(uarmysize)
		aspec := argl.rest()
		if !ok {
			fmt.Fprintf(os.Stderr, "! missing spec\n")
			return
		}
		var spec botspec
		switch len(aspec) {
		case 1:
			spec, ok = namedbotspecs[aspec[0]]
			if !ok {
				fmt.Fprintf(os.Stderr, "! spec named %q not found\n", aspec[0])
				return
			}
		case 3:
			spec, err = botparsea(aspec)
			if err != nil {
				fmt.Fprintf(os.Stderr, "! %v\n", err)
				return
			}
		default:
			fmt.Fprintf(os.Stderr, "! invalid spec length %d\n", len(aspec))
			return
		}
		ctx, cancel := context.WithCancel(context.Background())
		army, err := startarmy(ctx, cancel, armysize, spec, armyname, address)
		if err != nil {
			fmt.Fprintf(os.Stderr, "! failed to start the bot army: %v\n", err)
			return
		}
		fmt.Fprintf(os.Stderr, "< army %q spawned\n", armyname)
		c.armies[armyname] = army
	} else if cmd == "kill" {
		armyname, ok := argl.next()
		if !ok {
			fmt.Fprintf(os.Stderr, "! missing army name\n")
			return
		}
		army, ok := c.armies[armyname]
		if !ok {
			fmt.Fprintf(os.Stderr, "! army not found\n")
			return
		}
		army.cancel()
		delete(c.armies, armyname)
		fmt.Fprintf(os.Stderr, "< army %q killed\n", armyname)
	} else if cmd == "join" || cmd == "exit" {
		armyname, ok := argl.next()
		if !ok {
			fmt.Fprintf(os.Stderr, "! missing army name\n")
			return
		}
		army, ok := c.armies[armyname]
		if !ok {
			fmt.Fprintf(os.Stderr, "! army not found\n")
			return
		}
		sroom, ok := argl.next()
		if !ok {
			fmt.Fprintf(os.Stderr, "! missing room\n")
			return
		}
		uroom, err := strconv.ParseUint(sroom, 10, 32)
		if err != nil {
			fmt.Fprintf(os.Stderr, "! invalid room: %v\n", err)
			return
		}
		room := uint32(uroom)
		if cmd == "join" {
			name, ok := argl.next()
			if !ok {
				fmt.Fprintf(os.Stderr, "! missing user name\n")
				return
			}
			army.join(room, name)
			fmt.Fprintf(os.Stderr, "< army %q joined room %d with names starting with %q\n", armyname, room, name)
		} else {
			army.exit(room)
			fmt.Fprintf(os.Stderr, "< army %q exited room %d\n", armyname, room)
		}
	}
}

func (c console) handleclient(address string, cmd string, argl *arglist) {
	getid := func() (string, bool) {
		id, ok := argl.next()
		if !ok {
			fmt.Fprintf(os.Stderr, "! missing id for client\n")
			return "", false
		}
		return id, true
	}

	getcli := func() (conclient, bool) {
		id, ok := getid()
		if !ok {
			return conclient{}, false
		}
		cli, ok := c.clients[id]
		if !ok {
			fmt.Fprintf(os.Stderr, "! client %q not found\n", id)
			return conclient{}, false
		}
		return cli, true
	}

	getroom := func() (uint32, bool) {
		s, ok := argl.next()
		if !ok {
			fmt.Fprintf(os.Stderr, "! missing room\n")
			return 0, false
		}
		u, err := strconv.ParseUint(s, 10, 32)
		if err != nil {
			fmt.Fprintf(os.Stderr, "! invalid room %q: %v\n", s, err)
			return 0, false
		}
		return uint32(u), true
	}

	switch cmd {
	case "new":
		id, ok := getid()
		if !ok {
			return
		}
		if _, ok := c.clients[id]; ok {
			fmt.Fprintf(os.Stderr, "! there's another client with that id\n")
			return
		}
		rawconn, err := net.Dial("tcp", address)
		if err != nil {
			fmt.Fprintf(os.Stderr, "! failed to connect: %v\n", err)
			return
		}
		ctx, cancel := context.WithCancel(context.Background())
		pc := makeconn(ctx, cancel).
			start(rawconn, rawconn).
			withmiddleware(
				func(m protomes) { l.log(fmt.Sprintf("client/%v", id), m) },
				func(m protomes) { l.log(fmt.Sprintf("client/%v", id), m) },
			)
		cli := conclient{
			id:   id,
			pc:   pc,
			join: make(chan joinspec),
			exit: make(chan uint32),
			talk: make(chan talkspec),
		}
		c.clients[id] = cli
		go cli.handlemessages()
		go cli.main()
		fmt.Fprintf(os.Stderr, "< started client %q\n", id)
	case "rm":
		cli, ok := getcli()
		if ok {
			cli.pc.cancel()
			delete(c.clients, cli.id)
			fmt.Fprintf(os.Stderr, "< removed client %q\n", cli.id)
		}
	case "join":
		cli, ok := getcli()
		if !ok {
			return
		}
		room, ok := getroom()
		if !ok {
			return
		}
		name, ok := argl.next()
		if !ok {
			fmt.Fprintf(os.Stderr, "! missing name\n")
			return
		}
		js := joinspec{room, name}
		if !trysend(cli.join, js, cli.pc.ctx.Done()) {
			fmt.Fprintf(os.Stderr, "! client dead (?)\n")
		}
	case "exit":
		cli, ok := getcli()
		if !ok {
			return
		}
		room, ok := getroom()
		if !ok {
			return
		}
		if !trysend(cli.exit, room, cli.pc.ctx.Done()) {
			fmt.Fprintf(os.Stderr, "! client dead (?)\n")
		}
	case "talk":
		cli, ok := getcli()
		if !ok {
			return
		}
		room, ok := getroom()
		if !ok {
			return
		}
		atext := argl.rest()
		text := strings.Join(atext, " ")
		if len(text) == 0 {
			fmt.Fprintf(os.Stderr, "! missing text\n")
			return
		}
		if !trysend(cli.talk, talkspec{room, text}, cli.pc.ctx.Done()) {
			fmt.Fprintf(os.Stderr, "! client dead (?)\n")
		}
	default:
		fmt.Fprintf(os.Stderr, "! unknown command %q\n", cmd)
	}
}

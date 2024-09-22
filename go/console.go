package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type joinspec struct {
	room uint32
	name string
}

type talkspec struct {
	room uint32
	text string
}

type arglist []string

func newarglist(s string) *arglist {
	a := respace.Split(s, -1)
	return (*arglist)(&a)
}

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

type conclients map[string]conclient

type conarmies map[string]army

func prf(f string, a ...any) {
	fmt.Fprintf(os.Stderr, f, a...)
}

const logdateformat = "20060102_1504"

func conmain(address string, logname string, realtime bool) {
	shouldlog := len(logname) > 0
	var l logger
	if shouldlog {
		path := fmt.Sprintf("./logs/%s_%s.csv", logname, time.Now().Format(logdateformat))
		var err error
		file, err := os.Create(path)
		if err != nil {
			prf("failed to open %s", path)
			return
		}
		l = makelogger(csv.NewWriter(file))
		go l.main()
		prf("< logging to %q\n", path)
	}

	var cms chan connmes
	var cip *connidprovider
	if realtime {
		cip = newconnidprovider()
		cms = make(chan connmes)
		go checkrt(cms)
		prf("< running real-time checking\n")
	}

	makemw := func(s string) middleware {
		return func(m protomes) {
			if shouldlog {
				l.log(s, m)
			}
			if realtime {
				cid := cip.connidfor(s)
				cm := makeconnmes(cid, m)
				cms <- cm
			}
		}
	}
	startf := func(id string) {
		if shouldlog {
			l.enter(id)
		}
		if realtime {
			cid := cip.connidfor(id)
			cm := makeconnmes(cid, protomes{t: mconnstart})
			cms <- cm
		}
	}
	endf := func(id string) {
		if shouldlog {
			l.quit(id)
		}
		if realtime {
			cid := cip.connidfor(id)
			cm := makeconnmes(cid, protomes{t: mconnend})
			cms <- cm
		}
	}

	cc := make(conclients)
	ca := make(conarmies)

	sc := bufio.NewScanner(os.Stdin)

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for sc.Scan() {
		toks := respace.Split(sc.Text(), -1)
		if toks[0] == "quit" {
			prf("< bye\n")
			break
		}
		if len(toks) < 2 {
			prf("! <domain> <command> [<args>]\n")
			continue
		}
		domain, args := toks[0], toks[1:]
		argl0 := arglist(args)
		argl := &argl0

		// TODO: since this may take stdin from a file
		// maybe every loop should have a little bit of sleep, like 10ms
		// just to avoid overloading the server
		// maybe, idk

		if domain == "sleep" {

			handlesleep(argl)
			continue
		}

		cmd, ok := argl.next()
		if !ok {
			prf("! missing command\a")
			continue
		}
		switch domain {
		case "army":
			ca.handlearmy(address, cmd, argl, makemw, startf, endf)
		case "cli":
			cc.handleclient(address, cmd, argl, makemw, startf, endf)
		default:
			prf("! unknown domain %q\n", domain)
		}

		<-ticker.C
	}

	l.w.Flush()

	if err := sc.Err(); err != nil {
		prf("! scanner: %v\n", err)
	}
}

func handlesleep(argl *arglist) {
	sleeps, ok := argl.next()
	if !ok {
		prf("! missing sleep duration\n")
		return
	}
	sleep, err := time.ParseDuration(sleeps)
	if err != nil {
		prf("! invalid sleep duration: %v\n", err)
		return
	}
	time.Sleep(sleep)
}

func (ca conarmies) handlearmy(address string, cmd string, argl *arglist, f func(string) middleware, startf func(string), endf func(string)) {
	if cmd == "spawn" {
		armyname, ok := argl.next()
		if !ok {
			prf("! missing army name\n")
			return
		}
		sarmysize, ok := argl.next()
		if !ok {
			prf("! missing army size\n")
			return
		}
		uarmysize, err := strconv.ParseUint(sarmysize, 10, 32)
		if err != nil {
			prf("! invalid army size: %v\n", err)
			return
		}
		armysize := uint(uarmysize)
		aspec := argl.rest()
		if !ok {
			prf("! missing spec\n")
			return
		}
		var spec botspec
		switch len(aspec) {
		case 1:
			spec, ok = namedbotspecs[aspec[0]]
			if !ok {
				prf("! spec named %q not found\n", aspec[0])
				return
			}
		case 3:
			spec, err = botparsea(aspec)
			if err != nil {
				prf("! %v\n", err)
				return
			}
		default:
			prf("! invalid spec length %d\n", len(aspec))
			return
		}
		ctx, cancel := context.WithCancel(context.Background())
		army, err := startarmy(ctx, cancel, armysize, spec, armyname, address, f, startf, endf)
		if err != nil {
			prf("! failed to start the bot army: %v\n", err)
			return
		}
		prf("< army %q spawned\n", armyname)
		ca[armyname] = army
	} else if cmd == "kill" {
		armyname, ok := argl.next()
		if !ok {
			prf("! missing army name\n")
			return
		}
		army, ok := ca[armyname]
		if !ok {
			prf("! army not found\n")
			return
		}
		army.cancel()
		delete(ca, armyname)
		prf("< army %q killed\n", armyname)
	} else if cmd == "join" || cmd == "exit" {
		armyname, ok := argl.next()
		if !ok {
			prf("! missing army name\n")
			return
		}
		army, ok := ca[armyname]
		if !ok {
			prf("! army not found\n")
			return
		}
		sroom, ok := argl.next()
		if !ok {
			prf("! missing room\n")
			return
		}
		uroom, err := strconv.ParseUint(sroom, 10, 32)
		if err != nil {
			prf("! invalid room: %v\n", err)
			return
		}
		room := uint32(uroom)
		if cmd == "join" {
			name, ok := argl.next()
			if !ok {
				prf("! missing user name\n")
				return
			}
			army.join(room, name)
			prf("< army %q joined room %d with names starting with %q\n", armyname, room, name)
		} else {
			army.exit(room)
			prf("< army %q exited room %d\n", armyname, room)
		}
	}
}

// those callbacks are a little ugly, but ok
func (cc conclients) handleclient(address string, cmd string, argl *arglist, f func(string) middleware, startf func(string), endf func(string)) {
	getid := func() (string, bool) {
		id, ok := argl.next()
		if !ok {
			prf("! missing id for client\n")
			return "", false
		}
		return id, true
	}

	getcli := func() (conclient, bool) {
		id, ok := getid()
		if !ok {
			return conclient{}, false
		}
		cli, ok := cc[id]
		if !ok {
			prf("! client %q not found\n", id)
			return conclient{}, false
		}
		return cli, true
	}

	getroom := func() (uint32, bool) {
		s, ok := argl.next()
		if !ok {
			prf("! missing room\n")
			return 0, false
		}
		u, err := strconv.ParseUint(s, 10, 32)
		if err != nil {
			prf("! invalid room %q: %v\n", s, err)
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
		if len(id) == 0 {
			prf("! connection id cannot be empty\n")
			return
		}
		if _, ok := cc[id]; ok {
			prf("! there's another client with that id\n")
			return
		}
		rawconn, err := net.Dial("tcp", address)
		if err != nil {
			prf("! failed to connect: %v\n", err)
			return
		}

		logid := fmt.Sprintf("client/%v", id)

		ctx, cancel := context.WithCancel(context.Background())
		if startf != nil {
			startf(logid)
		}
		if endf != nil {
			context.AfterFunc(ctx, func() { endf(logid) })
		}

		pc := makeconn(ctx, cancel).
			start(rawconn, rawconn).
			withmiddleware(f(logid))
		cli := conclient{
			id:   id,
			pc:   pc,
			join: make(chan joinspec),
			exit: make(chan uint32),
			talk: make(chan talkspec),
		}
		cc[id] = cli
		go cli.handlemessages()
		go cli.main()
		prf("< started client %q\n", id)
	case "rm":
		cli, ok := getcli()
		if ok {
			cli.pc.cancel()
			delete(cc, cli.id)
			prf("< removed client %q\n", cli.id)
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
			prf("! missing name\n")
			return
		}
		js := joinspec{room, name}
		if !trysend(cli.join, js, cli.pc.ctx.Done()) {
			prf("! client dead (?)\n")
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
			prf("! client dead (?)\n")
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
			prf("! missing text\n")
			return
		}
		if !trysend(cli.talk, talkspec{room, text}, cli.pc.ctx.Done()) {
			prf("! client dead (?)\n")
		}
	default:
		prf("! unknown command %q\n", cmd)
	}
}

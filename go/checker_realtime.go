package main

import (
	"bufio"
	"context"
	"os"
	"slices"
	"strings"
	"time"
)

type fulwaiter struct {
	ctx    context.Context
	cancel context.CancelFunc
	cmi    int
	fqs    chan fulquestion
	casts  []connmes
}

type fulquestion struct {
	cm   connmes
	fuld chan bool
}

func rtcheckmain(address string) {
	cc := make(conclients)
	ca := make(conarmies)

	cids := make(map[string]connid)
	nextcid := 1
	getcid := func(s string) connid {
		cid, ok := cids[s]
		if !ok {
			cid = nextcid
			cids[s] = cid
			nextcid++
		}
		return cid
	}

	cms := make(chan connmes)
	go checkrt(cms)

	makemw := func(s string) middleware {
		cid := getcid(s)
		return func(m protomes) {
			cms <- makeconnmes(cid, m)
		}
	}
	startf := func(s string) {
		cid := getcid(s)
		cms <- makeconnmes(cid, protomes{t: mbegc})
	}
	endf := func(s string) {
		cid := getcid(s)
		cms <- makeconnmes(cid, protomes{t: mendc})
	}

	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		s := sc.Text()
		if len(s)-2 == strings.Index(s, "\\c") {
			prf("< command cancelled\n")
			continue
		}
		if s == "quit" {
			prf("< bye\n")
			break
		}
		argl := newarglist(sc.Text())
		domain, ok := argl.next()
		if !ok {
			prf("! missing domain\n")
		}
		if domain == "sleep" {
			handlesleep(argl)
			continue
		}
		cmd, ok := argl.next()
		if !ok {
			prf("! missing command\n")
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

	}
}

func checkrt(cms <-chan connmes) {
	const fulfillTimeout = 5 * time.Second

	sim := makeSimulation()

	cmlist := ([]connmes)(nil)

	// what do I mean by "fulfillment" here?
	// e.g. client joins room -> the "join" message gets "fulfilled" when every other room member receives a corresponding "jned" message

	fws := make(map[int]fulwaiter)

	fuldc := make(chan int)
	unfuldc := make(chan int)

	for {
		select {
		case cmi := <-fuldc:
			if cm := cmlist[cmi]; cm.t != mping {
				prf("< (rt) fulfilled: [%d] %v\n", cmi, cm)
			}
			delete(fws, cmi)
		case cmi := <-unfuldc:
			prf("! (rt) unfulfilled: [%d] %v\n", cmi, cmlist[cmi])
			delete(fws, cmi)
		case cm := <-cms:
			i := len(cmlist)
			cmlist = append(cmlist, cm)

			casts, needsful := sim.handle(cm)

			if cm.t == mbegc {
				// doesn't need fulfillment but also doesn't fulfill
				continue
			} else if needsful {
				if len(casts) == 0 {
					continue
				}
				deadline := time.Now().Add(fulfillTimeout)
				ctx, cancel := context.WithDeadline(context.Background(), deadline)
				fw := fulwaiter{
					ctx:    ctx,
					cancel: cancel,
					fqs:    make(chan fulquestion),
					cmi:    i,
					casts:  casts,
				}
				fws[i] = fw
				go fw.main(fuldc, unfuldc)
			} else {
				// if it doesn't need fulfillment, then it fulfills some previous message
				fuldcmi := -1
				fq := fulquestion{cm, make(chan bool)}
				for _, fw := range fws {
					if fuldcmi != -1 {
						break
					}
					select {
					case <-fw.ctx.Done():
					case fw.fqs <- fq:
						if <-fq.fuld {
							fuldcmi = fw.cmi
						}
					}
				}
				if fuldcmi == -1 {
					prf("! (rt) doesn't fulfill: %v\n", cm)
				} else if fuldcm := cmlist[fuldcmi]; fuldcm.t != mping {
					// prf("< cast %v fulfills %v\n", cm, fuldcm)
				}
			}

		}
	}
}

func (fw fulwaiter) main(fuldc chan<- int, unfuldc chan<- int) {
	fuld := make([]bool, len(fw.casts))
	context.AfterFunc(fw.ctx, func() {
		if slices.Contains(fuld, false) {
			unfuldc <- fw.cmi // TODO: return which casts are missing
		} else {
			fuldc <- fw.cmi
		}
	})
	defer fw.cancel()
	if len(fw.casts) == 0 {
		return
	}

	for {
		select {
		case <-fw.ctx.Done():
			return
		case fq := <-fw.fqs:
			if j := slices.IndexFunc(fw.casts, fq.cm.equal); j >= 0 {
				fq.fuld <- true
				fuld[j] = true
				if !slices.Contains(fuld, false) {
					return
				}
			} else {
				fq.fuld <- false
			}
		}
	}

}

package main

import (
	"context"
	"slices"
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

func checkrt(cms <-chan connmes, done chan<- zero) {
	defer close(done)
	const fulfillTimeout = 5 * time.Second

	sim := makeSimulation()

	cmlist := ([]connmes)(nil)

	// what do I mean by "fulfillment" here?
	// e.g. client joins room -> the "join" message gets "fulfilled" when every other room member receives a corresponding "jned" message

	fws := make(map[int]fulwaiter)

	fuldc := make(chan int)
	unfuldc := make(chan int)

	unfuls := ([]int)(nil)
	nofuls := ([]int)(nil)

loop:
	for {
		select {
		case cmi := <-fuldc:
			if cm := cmlist[cmi]; cm.t != mping {
				// prf("< (rt) fulfilled: [%d] %v\n", cmi, cm)
			}
			delete(fws, cmi)
		case cmi := <-unfuldc:
			// prf("! (rt) unfulfilled: [%d] %v\n", cmi, cmlist[cmi])
			unfuls = append(unfuls, cmi)
			delete(fws, cmi)
		case cm, ok := <-cms:
			if !ok {
				break loop
			}
			i := len(cmlist)
			cmlist = append(cmlist, cm)

			casts, needsful := sim.handle(cm)

			if needsful {
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
				if cm.t == mbegc {
					// except this one
					// it doesn't need fulfillment but also doesn't fulfill
					continue
				}
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
					// prf("! (rt) doesn't fulfill: %v\n", cm)
					nofuls = append(nofuls, i)
				} else {
					fuldcm := cmlist[fuldcmi]
					if fuldcm.t != mping {
						// prf("< (rt) cast %v\n", cm)
						// prf("       fulfills [%d] %v\n", fuldcmi, fuldcm)
					}
				}
			}

		}
	}

	if len(unfuls) > 0 {
		prf("\n")
		prf("(rt) %03d message(s) didn't get fulfilled:\n", len(unfuls))
		for _, i := range unfuls {
			prf("     %v\n", cmlist[i])
		}
	} else {
		prf("\n(rt) all messages got fulfilled!\n")
	}

	if len(nofuls) > 0 {
		prf("\n")
		prf("(rt) %03d message(s) didn't fulfill any previous message:\n", len(nofuls))
		for _, i := range nofuls {
			prf("     %v\n", cmlist[i])
		}
	} else {
		prf("\n(rt) all messages did fulfill!\n")
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

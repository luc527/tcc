package chk

// TODO: have to re-test the realtime checker

import (
	"context"
	"slices"
	"tccgo/mes"
	"time"
)

type fulwaiter struct {
	ctx    context.Context
	cancel context.CancelFunc
	cmi    int
	fqs    chan fulquestion
	casts  []ConnMessage
}

type fulquestion struct {
	cm   ConnMessage
	fuld chan bool
}

type RtcheckResults struct {
	Messages []ConnMessage
	Ok       []bool
}

type Rtchecker struct {
	done chan zero
	cms  chan ConnMessage
	res  RtcheckResults
}

func NewRtchecker() *Rtchecker {
	return &Rtchecker{
		done: make(chan zero),
		cms:  make(chan ConnMessage),
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

func (rtc *Rtchecker) Send(cm ConnMessage) {
	select {
	case rtc.cms <- cm:
	case <-rtc.done:
	}
}

func (rtc *Rtchecker) Start() {
	go rtc.main()
}

func (rtc *Rtchecker) Stop() {
	close(rtc.cms)
}

func (rtc *Rtchecker) Results() RtcheckResults {
	<-rtc.done
	return rtc.res
}

func (rtc *Rtchecker) main() {
	defer close(rtc.done)
	const fulfillTimeout = 5 * time.Second

	sim := MakeSimulation()

	cmlist := ([]ConnMessage)(nil)
	oks := ([]bool)(nil)

	// what do I mean by "fulfillment" here?
	// e.g. client joins room -> the "join" message gets "fulfilled" when every other room member receives a corresponding "jned" message

	fws := make(map[int]fulwaiter)

	fuldc := make(chan int)
	unfuldc := make(chan int)

loop:
	for {
		select {
		case cmi := <-fuldc:
			if cm := cmlist[cmi]; cm.T != mes.PingType {
			}
			delete(fws, cmi)
			oks[cmi] = true
		case cmi := <-unfuldc:
			delete(fws, cmi)
		case cm, ok := <-rtc.cms:
			if !ok {
				break loop
			}
			i := len(cmlist)
			cmlist = append(cmlist, cm)
			oks = append(oks, false)

			casts, needsful := sim.Handle(cm)

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
				if cm.T == mes.BegcType {
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
				if fuldcmi != -1 {
					oks[i] = true
				}
			}

		}
	}

	rtc.res.Messages = cmlist
	rtc.res.Ok = oks
}

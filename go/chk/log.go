package chk

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"slices"
)

type messageIndex struct {
	is []int
	ms []ConnMessage
}

func (mi messageIndex) after(idx int) []ConnMessage {
	locidx, found := slices.BinarySearch(mi.is, idx)
	from := locidx
	if found {
		from = locidx + 1
	}
	return mi.ms[from:]
}

func buildMessageIndexes(cms []ConnMessage) map[connId]messageIndex {
	mis := make(map[connId]messageIndex)
	for i, cm := range cms {
		cid := cm.ConnId
		mi := mis[cid]
		mis[cid] = messageIndex{
			is: append(mi.is, i),
			ms: append(mi.ms, cm),
		}
	}
	return mis
}

type LogcheckResults struct {
	Messages []logmes
	Want     [][]ConnMessage
	Got      [][]int // parallel to Want; index into Messages, otherwise -1
}

func CheckLog(r0 io.Reader) LogcheckResults {
	r := csv.NewReader(os.Stdin)
	r.Read() // ignore header

	lms := make([]logmes, 0, 256)
	castsWant := make([][]ConnMessage, 0, 256)
	castsGot := make([][]int, 0, 256)

	for {
		rec, err := r.Read()
		if err != nil {
			if err != io.EOF {
				log.Println(err)
			}
			break
		}
		lm := logmes{}
		if err := lm.fromrecord(rec); err != nil {
			log.Println("skipping, err:", err)
			continue
		}
		lms = append(lms, lm)
	}

	slices.SortFunc(lms, func(a logmes, b logmes) int {
		return int(a.dur - b.dur)
	})

	cip := NewConnIdProvider()

	cms := make([]ConnMessage, len(lms))

	for i, lm := range lms {
		cid := cip.ConnIdFor(lm.connName)
		cm := MakeConnMessage(cid, lm.m)
		cms[i] = cm
	}

	// the strategy is:
	// for each client-sent message, figure out which messages the server should send in response,
	// and check whether all of those messages appear after the client-sent message

	// each connection has a message index to look for the messager after
	connmi := buildMessageIndexes(cms)

	sim := MakeSimulation()
	for cmi, cm := range cms {
		// "casts" are the messages the server broadcasts to the clients in response
		// to a message from a client. although the "pong" is more of a response
		// than a cast.
		wants, _ := sim.Handle(cm)
		gots := make([]int, len(wants))

		for wi, w := range wants {
			mi, ok := connmi[w.ConnId]
			gots[wi] = math.MinInt
			if ok {
				ii := slices.IndexFunc(mi.after(cmi), w.equalmes)
				gots[wi] = mi.is[ii]
				if !slices.ContainsFunc(mi.after(cmi), w.equalmes) {
					fmt.Printf("\t%v\n", w)
				}
			} else {
				// TODO: handle better
				log.Fatalf("messageIndex not found for conn id %v", w.ConnId)
			}
		}

		castsWant = append(castsWant, wants)
		castsGot = append(castsGot, gots)
	}

	return LogcheckResults{
		Messages: lms,
		Want:     castsWant,
		Got:      castsGot,
	}
}

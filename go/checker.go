package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"slices"
)

type mesindex struct {
	idxs []int
	ms   []connmes
}

func (mi mesindex) after(idx int) []connmes {
	locidx, found := slices.BinarySearch(mi.idxs, idx)
	from := locidx
	if found {
		from = locidx + 1
	}
	return mi.ms[from:]
}

func checkmain() {
	r := csv.NewReader(os.Stdin)
	r.Read() // ignore header

	lms := make([]logmes, 0, 1024)

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

	cip := newconnidprovider()

	connmi := make(map[connid]mesindex)
	cms := make([]connmes, len(lms))

	for i, lm := range lms {
		cid := cip.connidfor(lm.connName)
		cm := makeconnmes(cid, lm.m)
		cms[i] = cm

		mi := connmi[cid] // zero value usable
		connmi[cid] = mesindex{
			idxs: append(mi.idxs, i),
			ms:   append(mi.ms, cm),
		}
	}

	errors := 0

	sim := makeSimulation()
	for i, cm := range cms {
		// "casts" are the messages the server broadcasts to the clients in response
		// to a message from a client. although the "pong" is more of a response
		// than a cast.
		casts, _ := sim.handle(cm)

		anymissing := false
		for _, cast := range casts {
			mi, ok := connmi[cast.cid]
			if !ok {
				log.Fatalf("mesindex not found for conn id %v", cast.cid)
			}
			if !slices.ContainsFunc(mi.after(i), cast.equalmes) {
				if !anymissing {
					anymissing = true
					fmt.Printf("missing after (%d) %v:\n", i, cm)
				}
				fmt.Printf("\t%v\n", cast)
			}
		}
		if !anymissing && len(casts) > 0 {
			fmt.Printf("(cm %3d) all %3d expected casts were sent\n", i, len(casts))
		}
		if anymissing {
			errors++
		}
	}

	fmt.Printf("errors: %v\n", errors)
}

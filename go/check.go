package main

import (
	"encoding/csv"
	"io"
	"log"
	"os"
	"slices"
)

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

	slices.SortFunc(lms, func(a logmes, b logmes) int { return int(a.dur - b.dur) })

	for _, lm := range lms {
		fmt.Println(logepoch.Add(lm.dur))
	}
	fmt.Printf("# messages: %v\n", len(lms))
}

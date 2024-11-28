package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

// TODO: also check for missing messages

type entry struct {
	topic       uint16
	publisher   uint16
	publication uint16
}

func (e entry) eq(o entry) bool {
	return e.topic == o.topic &&
		e.publisher == o.publisher &&
		e.publication == o.publication
}

type timedEntries struct {
	entries []entry
	usecs   []int64
}

func (te *timedEntries) append(usec int64, e entry) {
	te.entries = append(te.entries, e)
	te.usecs = append(te.usecs, usec)
}

func (te *timedEntries) length() int {
	return len(te.entries)
}

func mustParseInt(s string) int64 {
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		log.Fatalf("failed parse: %v", err)
	}
	return i
}

func main() {
	if len(os.Args) != 3 {
		log.Fatal("missing lang and date")
	}

	lang := os.Args[1]
	date := os.Args[2]

	path := fmt.Sprintf("data/latency_%s_cli_%s.txt", lang, date)
	in, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer in.Close()

	latencyPath := fmt.Sprintf("data/latency_%s_latencies_%s.csv", lang, date)
	latencyOut, err := os.OpenFile(latencyPath, os.O_CREATE|os.O_WRONLY, os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}
	defer latencyOut.Close()

	itersPath := fmt.Sprintf("data/latency_%s_iters_%s.csv", lang, date)
	itersOut, err := os.OpenFile(itersPath, os.O_CREATE|os.O_WRONLY, os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}
	defer itersOut.Close()

	log.Printf("latencies: %v", latencyPath)
	log.Printf("iters:     %v", itersPath)

	var (
		reLine      = regexp.MustCompile(`(\w+): (\d+) (.*)`)
		reIteration = regexp.MustCompile(`(\d+) subs per topic, (.*) connections`)
		reEntry     = regexp.MustCompile(`topic=(\d+) payload=pubsher (\d+), pubton (\d+)`)
	)

	pubEntries := timedEntries{}
	subEntries := timedEntries{}

	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		line := scanner.Text()
		lineParts := reLine.FindStringSubmatch(line)

		if len(lineParts) != 4 {
			log.Printf("weird line: %q", line)
			continue
		}

		dbg := lineParts[1] == "dbg"
		pub := lineParts[1] == "pub"
		sub := lineParts[1] == "sub"

		timestamp, err := strconv.ParseInt(lineParts[2], 10, 64)
		if err != nil {
			log.Fatalf("invalid timestamp: %v", err)
		}

		rest := lineParts[3]

		if dbg {
			iterParts := reIteration.FindStringSubmatch(line)
			if len(iterParts) == 0 {
				continue
			}
			subs, err := strconv.ParseInt(iterParts[1], 10, 64)
			if err != nil {
				log.Fatal(err)
			}
			var conns int64
			if iterParts[2] != "reusing" {
				ss := strings.Split(iterParts[2], " ")
				conns, err = strconv.ParseInt(ss[0], 10, 64)
				if err != nil {
					log.Fatal(err)
				}
			}
			fmt.Fprintf(itersOut, "%d,%d,%d\n", timestamp/1000/1000, subs, conns)
			// ignoring dbg for now
		} else if pub || sub {
			entryParts := reEntry.FindStringSubmatch(rest)
			if len(entryParts) != 4 {
				log.Printf("weird entry: %q", rest)
				continue
			}
			topic := mustParseInt(entryParts[1])
			publication := mustParseInt(entryParts[2])
			publisher := mustParseInt(entryParts[3])
			e := entry{
				topic:       uint16(topic),
				publication: uint16(publication),
				publisher:   uint16(publisher),
			}
			if pub {
				pubEntries.append(timestamp, e)
			} else if sub {
				subEntries.append(timestamp, e)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("scanner: %v", err)
	}

	runtime.GC()

	for i := range pubEntries.entries {
		pubEntry := pubEntries.entries[i]
		pubUsec := pubEntries.usecs[i]

		for j := range subEntries.entries {
			subEntry := subEntries.entries[j]
			if pubEntry.eq(subEntry) {
				subUsec := subEntries.usecs[j]
				delayUsec := subUsec - pubUsec
				timestampSec := pubUsec / 1000 / 1000
				fmt.Fprintf(latencyOut, "%d,%d,%d\n", timestampSec, pubEntry.topic, delayUsec)
			}
		}
	}
}

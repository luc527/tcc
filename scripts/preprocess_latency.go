package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"regexp"
	"runtime"
	"strconv"
)

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
	var (
		reLine  = regexp.MustCompile(`(\w+): (\d+) (.*)`)
		reEntry = regexp.MustCompile(`unix_usec=(\d+) topic=(\d+) payload=publisher (\d+), publication (\d+)`)
	)

	pubEntries := timedEntries{}
	subEntries := timedEntries{}

	scanner := bufio.NewScanner(os.Stdin)
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
		_ = timestamp

		rest := lineParts[3]

		if dbg {
			// ignoring dbg for now
			// TODO: count failures to read/subscribe/etc. too
			// (!too many for node)
		} else if pub || sub {
			entryParts := reEntry.FindStringSubmatch(rest)
			if len(entryParts) != 5 {
				log.Printf("weird entry: %q", rest)
				continue
			}
			usec := mustParseInt(entryParts[1])
			topic := mustParseInt(entryParts[2])
			publication := mustParseInt(entryParts[3])
			publisher := mustParseInt(entryParts[4])
			e := entry{
				topic:       uint16(topic),
				publication: uint16(publication),
				publisher:   uint16(publisher),
			}
			if pub {
				pubEntries.append(usec, e)
			} else if sub {
				subEntries.append(usec, e)
			} else {
				log.Fatal("weird type 1")
			}
		} else {
			log.Fatal("weird type 2")
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
				fmt.Printf("%d,%d,%d\n", timestampSec, pubEntry.topic, delayUsec)
			}
		}
	}
}

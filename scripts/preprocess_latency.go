package main

import (
	"bufio"
	"fmt"
	"log"
	"maps"
	"os"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
)

type entry struct {
	topic       uint16
	publisher   uint16
	publication uint16
}

func (e entry) cmp(o entry) int {
	if d := int(e.topic) - int(o.topic); d != 0 {
		return d
	}
	if d := int(e.publisher) - int(o.publisher); d != 0 {
		return d
	}
	if d := int(e.publication) - int(o.publication); d != 0 {
		return d
	}
	return 0
}

type entryTimed struct {
	timestamp int64
	entry
}

func (e entryTimed) cmp(o entryTimed) int {
	if d := e.entry.cmp(o.entry); d != 0 {
		return d
	}
	if d := e.timestamp - o.timestamp; d != 0 {
		return int(d)
	}
	return 0
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

	statsPath := fmt.Sprintf("data/latency_%s_statistics_%s.txt", lang, date)
	statsOut, err := os.OpenFile(statsPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}
	defer statsOut.Close()

	itersPath := fmt.Sprintf("data/latency_%s_iters_%s.csv", lang, date)
	itersOut, err := os.OpenFile(itersPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}
	defer itersOut.Close()

	log.Printf("statistics: %v", statsPath)
	log.Printf("iters:      %v", itersPath)

	var (
		reLine      = regexp.MustCompile(`(\w+): (\d+) (.*)`)
		reIteration = regexp.MustCompile(`(\d+) subs per topic, (.*) connections`)
		reEntry     = regexp.MustCompile(`topic=(\d+) payload=pubsher (\d+), pubton (\d+)`)
	)

	pubEntries := []entryTimed(nil)
	subEntries := []entryTimed(nil)

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
		} else if pub || sub {
			entryParts := reEntry.FindStringSubmatch(rest)
			if len(entryParts) != 4 {
				log.Printf("weird entry: %q", rest)
				continue
			}
			topic := mustParseInt(entryParts[1])
			publication := mustParseInt(entryParts[2])
			publisher := mustParseInt(entryParts[3])
			e := entryTimed{
				timestamp: timestamp,
				entry: entry{
					topic:       uint16(topic),
					publication: uint16(publication),
					publisher:   uint16(publisher),
				},
			}
			if pub {
				pubEntries = append(pubEntries, e)
			} else if sub {
				subEntries = append(subEntries, e)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("scanner: %v", err)
	}

	runtime.GC()

	slices.SortFunc(pubEntries, entryTimed.cmp)
	slices.SortFunc(subEntries, entryTimed.cmp)

	data := make(map[int64][]int)

	notfound := 0

	for i := range pubEntries {
		pubEntry := pubEntries[i]

		j, found := slices.BinarySearchFunc(
			subEntries,
			pubEntry.entry,
			func(e entryTimed, t entry) int {
				return e.entry.cmp(t)
			},
		)
		if !found {
			notfound++
			continue
		}
		subEntry := subEntries[j]

		sec := pubEntry.timestamp / 1000 / 1000
		latency := int(subEntry.timestamp - pubEntry.timestamp)

		data[sec] = append(data[sec], latency)
	}

	log.Printf("did not find receives for %d out of %d publications", notfound, len(pubEntries))

	timestamps := slices.Sorted(maps.Keys(data))
	min := make(map[int64]int)
	max := make(map[int64]int)
	mean := make(map[int64]int)
	median := make(map[int64]int)
	p90 := make(map[int64]int)
	p95 := make(map[int64]int)
	p99 := make(map[int64]int)

	for _, t := range timestamps {
		lats := data[t]
		slices.Sort(lats)
		min[t] = lats[0]
		max[t] = lats[len(lats)-1]

		if len(lats)%2 == 0 {
			i := len(lats) / 2
			median[t] = (lats[i] + lats[i+1]) / 2
		} else {
			i := len(lats) / 2
			median[t] = lats[i]
		}

		i90 := int(0.90 * float64(len(lats)))
		i95 := int(0.95 * float64(len(lats)))
		i99 := int(0.99 * float64(len(lats)))
		p90[t] = lats[i90]
		p95[t] = lats[i95]
		p99[t] = lats[i99]

		sum := int64(0)
		for _, lat := range lats {
			sum += int64(lat)
		}
		mean_ := float64(sum) / float64(len(lats))
		mean[t] = int(mean_)
	}

	writem := func(name string, m map[int64]int) {
		if _, err := fmt.Fprintf(statsOut, "#%s\n", name); err != nil {
			log.Fatal(err)
		}
		for _, t := range timestamps {
			if _, err := fmt.Fprintf(statsOut, "%d,%d\n", t, m[t]); err != nil {
				log.Fatal(err)
			}
		}
	}

	writem("min", min)
	writem("max", max)
	writem("mean", mean)
	writem("median", median)
	writem("p90", p90)
	writem("p95", p95)
	writem("p99", p99)
}

package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"
)

var (
	logepoch  time.Time
	logheader = []string{
		"conn",
		"nsec",
		"type",
		"room",
		"name",
		"text",
	}
)

func init() {
	var err error
	logepoch, err = time.ParseInLocation(time.DateOnly, "2024-01-01", time.Local)
	if err != nil {
		log.Fatal(err)
	}
}

type logmes struct {
	connName string
	dur      time.Duration
	m        protomes
}

func (lm logmes) String() string {
	return fmt.Sprintf("{%v, %v, %v}", lm.connName, logepoch.Add(lm.dur).Format("01/06 15:04:05.000000"), lm.m)
}

type logger struct {
	ctx    context.Context
	cancel context.CancelFunc
	w      *csv.Writer
	recs   chan []string
}

func makelogger(w *csv.Writer) logger {
	ctx, cancel := context.WithCancel(context.Background())
	context.AfterFunc(ctx, w.Flush)
	recs := make(chan []string, 16)
	return logger{ctx, cancel, w, recs}
}

func (lm logmes) torecord() []string {
	return []string{
		lm.connName,
		strconv.FormatInt(lm.dur.Nanoseconds(), 10),
		lm.m.t.String(),
		strconv.FormatUint(uint64(lm.m.room), 10),
		lm.m.name,
		lm.m.text,
	}
}

func (lm *logmes) fromrecord(rec []string) error {
	if len(rec) != 6 {
		return fmt.Errorf("logmes: invalid record length %d", len(rec))
	}

	connName := rec[0]
	snsec := rec[1]
	smtype := rec[2]
	sroom := rec[3]
	name := rec[4]
	text := rec[5]

	nsec, err := strconv.ParseInt(snsec, 10, 64)
	if err != nil {
		return fmt.Errorf("logmes: %w", err)
	}
	dur := time.Duration(nsec * int64(time.Nanosecond))

	t, err := parseMtype(smtype)
	if err != nil {
		return fmt.Errorf("logmes: %w", err)
	}

	room := uint32(0)
	if t.hasroom() {
		uroom, err := strconv.ParseUint(sroom, 10, 32)
		if err != nil {
			return fmt.Errorf("logmes: %w", err)
		}
		room = uint32(uroom)
	}

	lm.connName = connName
	lm.dur = dur
	lm.m.room = room
	lm.m.t = t
	lm.m.name = name
	lm.m.text = text
	return nil
}

func (l logger) main() {
	defer l.cancel()
	if err := l.w.Write(logheader); err != nil {
		fmt.Fprintln(os.Stderr, "failed to log header")
		return
	}
	for {
		select {
		case <-l.ctx.Done():
			fmt.Fprintln(os.Stderr, "! logger closed")
			return
		case rec := <-l.recs:
			if err := l.w.Write(rec); err != nil {
				fmt.Fprintln(os.Stderr, "failed to log in:", err)
				return
			}
		}
	}
}

func (l logger) log(connName string, m protomes) {
	lm := logmes{connName, time.Since(logepoch), m}
	rec := lm.torecord()
	if !trysend(l.recs, rec, l.ctx.Done()) {
		fmt.Fprintln(os.Stderr, "failed to log")
	}
}

func (l logger) enter(connName string) {
	rec := make([]string, len(logheader))
	rec[0] = connName
	rec[1] = strconv.FormatInt(time.Since(logepoch).Nanoseconds(), 10)
	rec[2] = "begc"
	if !trysend(l.recs, rec, l.ctx.Done()) {
		fmt.Fprintln(os.Stderr, "failed to log begc")
	}
}

func (l logger) quit(connName string) {
	rec := make([]string, len(logheader))
	rec[0] = connName
	rec[1] = strconv.FormatInt(time.Since(logepoch).Nanoseconds(), 10)
	rec[2] = "endc"
	if !trysend(l.recs, rec, l.ctx.Done()) {
		fmt.Fprintln(os.Stderr, "failed to log begc")
	}
}

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

var logepoch time.Time

func init() {
	var err error
	logepoch, err = time.ParseInLocation(time.DateOnly, "2024-01-01", time.Local)
	if err != nil {
		log.Fatal(err)
	}
}

type logmes struct {
	connid string
	dur    time.Duration
	m      protomes
}

type logger struct {
	ctx    context.Context
	cancel context.CancelFunc
	w      *csv.Writer
	lms    chan logmes
}

func makelogger(w *csv.Writer) logger {
	ctx, cancel := context.WithCancel(context.Background())
	lms := make(chan logmes, 16)
	return logger{ctx, cancel, w, lms}
}

func (lm logmes) torecord() []string {
	return []string{
		lm.connid,
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

	connid := rec[0]
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

	uroom, err := strconv.ParseUint(sroom, 10, 32)
	if err != nil {
		return fmt.Errorf("logmes: %w", err)
	}
	room := uint32(uroom)

	t, err := parseMtype(smtype)
	if err != nil {
		return fmt.Errorf("logmes: %w", err)
	}

	lm.connid = connid
	lm.dur = dur
	lm.m.room = room
	lm.m.t = t
	lm.m.name = name
	lm.m.text = text
	return nil
}

func (l logger) main() {
	defer l.cancel()
	header := []string{
		"conn",
		"nsec",
		"type",
		"room",
		"name",
		"text",
	}
	if err := l.w.Write(header); err != nil {
		log.Printf("failed to log header")
		return
	}
	for {
		select {
		case <-l.ctx.Done():
			fmt.Fprintln(os.Stderr, "! logger closed")
			return
		case lm := <-l.lms:
			if err := l.w.Write(lm.torecord()); err != nil {
				log.Println("failed to log in:", err)
				return
			}
		}
	}
}

func (l logger) log(connid string, m protomes) {
	lm := logmes{connid, time.Since(logepoch), m}
	if !trysend(l.lms, lm, l.ctx.Done()) {
		log.Println("failed to log")
	}
}
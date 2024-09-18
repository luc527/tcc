package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"iter"
	"log"
	"maps"
	"os"
	"slices"
	"strings"
)

// id of a connection (user) in the simulation
type connid = int

// id of a message in the simulation
type mesid = int

// message along with an id of the connection from which it was sent OR which will receive it
type connmes struct {
	cid connid
	protomes
}

func (cm connmes) String() string {
	return fmt.Sprintf("{%d, %v}", cm.cid, cm.protomes)
}

func (this connmes) compare(that connmes) int64 {
	var c int64
	c = int64(this.cid) - int64(that.cid)
	if c != 0 {
		return c
	}
	c = int64(int8(this.t) - int8(that.t))
	if c != 0 {
		return c
	}
	if this.t.hasroom() {
		c = int64(this.room) - int64(that.room)
		if c != 0 {
			return c
		}
	}
	if this.t.hasname() {
		c = int64(strings.Compare(this.name, that.name))
		if c != 0 {
			return c
		}
	}
	if this.t.hastext() {
		c = int64(strings.Compare(this.text, that.text))
		if c != 0 {
			return c
		}
	}
	return 0
}

// graph of messages
type mesgraph struct {
	data []connmes
	adj  map[mesid][]mesid
}

func newMesgraph() *mesgraph {
	return &mesgraph{
		data: nil,
		adj:  make(map[mesid][]mesid),
	}
}

func (mg *mesgraph) register(m connmes) mesid {
	id := len(mg.data)
	mg.data = append(mg.data, m)
	return id
}

const (
	// to be used as the "from" part of the edge representing the first message of a connection
	nilmes = mesid(-1)
)

func (mg *mesgraph) edge(from, to mesid) {
	mg.adj[from] = append(mg.adj[from], to)
}

// the simulation doesn't build the actual messages,
// but gives enough information as return values to build them
type simulation struct {
	rooms map[uint32]map[connid]string
}

func makeSimulation() simulation {
	return simulation{
		rooms: make(map[uint32]map[connid]string),
	}
}

// returns simconnids of users who should receive the jned message
// also returns false if the user was already in the room, or the name is already in use; true otherwise
func (sim simulation) join(cid connid, rid uint32, name string) (iter.Seq[connid], bool) {
	room, ok := sim.rooms[rid]
	if !ok {
		room = make(map[connid]string)
		sim.rooms[rid] = room
	}
	if _, ok := room[cid]; ok {
		return nil, false
	}
	for _, othername := range room {
		if othername == name {
			return nil, false
		}
	}
	room[cid] = name
	return maps.Keys(room), true
}

// returns name of the user was using in the room
// and ids of users who should receive the exed message
// and false if the user wasn't actually in the room, or the room didn't even exist
func (sim simulation) exit(cid connid, rid uint32) (string, iter.Seq[connid], bool) {
	room, ok := sim.rooms[rid]
	if !ok {
		return "", nil, false
	}
	if name, ok := room[cid]; ok {
		delete(room, cid)
		return name, maps.Keys(room), true
	} else {
		return "", nil, false
	}
}

// returns name of the user has in the room
// and ids of users who should receive the hear message
// and false if the user isn't even in the room, or the room doesn't even exist
func (sim simulation) talk(cid connid, rid uint32) (string, iter.Seq[connid], bool) {
	room, ok := sim.rooms[rid]
	if !ok {
		return "", nil, false
	}
	if name, ok := room[cid]; ok {
		return name, maps.Keys(room), true
	}
	return "", nil, false
}

func (sim simulation) handlecall(m connmes) ([]connmes, error) {
	switch m.t {
	case mping:
		pong := protomes{t: mpong}
		return []connmes{{m.cid, pong}}, nil
	case mjoin:
		receivers, ok := sim.join(m.cid, m.room, m.name)
		if ok {
			jned := protomes{t: mjned, room: m.room, name: m.name}
			return addconnids(receivers, jned), nil
		}
	case mexit:
		name, receivers, ok := sim.exit(m.cid, m.room)
		if ok {
			exed := protomes{t: mexed, room: m.room, name: name}
			return addconnids(receivers, exed), nil
		}
	case mtalk:
		name, receivers, ok := sim.talk(m.cid, m.room)
		if ok {
			hear := protomes{t: mhear, room: m.room, name: name, text: m.text}
			return addconnids(receivers, hear), nil
		}
	}
	return nil, fmt.Errorf("not a call message: %v", m.t)
}

func addconnids(cids iter.Seq[connid], m protomes) []connmes {
	cms := make([]connmes, 0)
	for cid := range cids {
		cm := connmes{cid, m}
		cms = append(cms, cm)
	}
	return cms
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

	nextcid := 1
	connids := make(map[string]connid)

	cms := make([]connmes, len(lms))
	for i, lm := range lms {
		cid, ok := connids[lm.connName]
		if !ok {
			cid = nextcid
			nextcid++
			connids[lm.connName] = cid
		}
		cms[i] = connmes{
			cid:      cid,
			protomes: lm.m,
		}
	}

	var calls []connmes

	for _, cm := range cms {
		if cm.t.iscall() {
			calls = append(calls, cm)
		}
	}

	sim := makeSimulation()
	for _, call := range calls {
		fmt.Printf("%v\n", call)
		casts, err := sim.handlecall(call)
		if err != nil {
			log.Fatal(err)
		}
		for _, cast := range casts {
			fmt.Printf("\t%v\n", cast)
		}
	}
}

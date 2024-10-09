package main

import (
	"fmt"
	"iter"
	"maps"
	"slices"
	"strings"
	"sync"
	"unique"
)

var (
	errNotOnline = fmt.Errorf("no connection with this id has begun")
)

// id of a connection (user) in the simulation
type connid = int

// message along with an id of the connection from which it was sent OR which will receive it
type connmes struct {
	cid  connid
	t    mtype
	room uint32
	name unique.Handle[string]
	text unique.Handle[string]
}

type connidprovider struct {
	mu sync.Mutex
	m  map[string]connid
}

func newconnidprovider() *connidprovider {
	return &connidprovider{
		sync.Mutex{},
		make(map[string]connid),
	}
}

func (cip *connidprovider) connidfor(s string) connid {
	cip.mu.Lock()
	defer cip.mu.Unlock()
	cid, ok := cip.m[s]
	if !ok {
		cid = len(cip.m)
		cip.m[s] = cid
	}
	return cid
}

// only create connmes through this function
// otherwise, you might leave name and text as the unique.Handle zero value, which contains a nil pointer which is dereferenced when you call .Value()
func makeconnmes(cid connid, m protomes) connmes {
	return connmes{
		cid:  cid,
		t:    m.t,
		room: m.room,
		name: unique.Make(m.name),
		text: unique.Make(m.text),
	}
}

func (cm connmes) String() string {
	return fmt.Sprintf("{%d, %v}", cm.cid, protomes2string(cm.t, cm.room, cm.name.Value(), cm.text.Value()))
}

func (cm connmes) protomes() protomes {
	return protomes{
		t:    cm.t,
		room: cm.room,
		name: cm.name.Value(),
		text: cm.name.Value(),
	}
}

func (this connmes) equal(that connmes) bool {
	if this.cid != that.cid {
		return false
	}
	return this.equalmes(that)
}

func (this connmes) equalmes(that connmes) bool {
	// ignores connid
	if this.t != that.t {
		return false
	}
	if this.t.hasroom() && this.room != that.room {
		return false
	}
	if this.t.hasname() && this.name != that.name {
		return false
	}
	if this.t.hastext() && this.text != that.text {
		return false
	}
	return true
}

// the simulation doesn't build the actual messages,
// but gives enough information as return values to build them
type simulation struct {
	online map[connid]zero
	rooms  map[uint32]map[connid]string
}

func makeSimulation() simulation {
	return simulation{
		online: make(map[connid]zero),
		rooms:  make(map[uint32]map[connid]string),
	}
}

func (sim simulation) startconn(cid connid) bool {
	if _, ok := sim.online[cid]; ok {
		return false
	}
	sim.online[cid] = zero{}
	return true
}

func (sim simulation) join(cid connid, rid uint32, name string) (iter.Seq[connid], error) {
	if _, ok := sim.online[cid]; !ok {
		return nil, errNotOnline
	}
	if !isnamevalid(name) {
		return nil, ebadname
	}
	nrooms := len(sim.clirooms(cid))
	if nrooms >= clientMaxRooms {
		return nil, eroomlimit
	}
	room, ok := sim.rooms[rid]
	if !ok {
		room = make(map[connid]string)
		sim.rooms[rid] = room
	}
	if _, ok := room[cid]; ok {
		return nil, ejoined
	}
	if len(room) >= roomCapacity {
		return nil, eroomfull
	}
	for _, othername := range room {
		if othername == name {
			return nil, enameinuse
		}
	}
	room[cid] = name
	return maps.Keys(room), nil
}

func (sim simulation) exit(cid connid, rid uint32) (string, iter.Seq[connid], error) {
	room, ok := sim.rooms[rid]
	if !ok {
		return "", nil, ebadroom
	}
	if name, ok := room[cid]; ok {
		delete(room, cid)
		return name, maps.Keys(room), nil
	} else {
		return "", nil, ebadroom
	}
}

func (sim simulation) talk(cid connid, text string, rid uint32) (string, iter.Seq[connid], error) {
	if _, ok := sim.online[cid]; !ok {
		return "", nil, errNotOnline
	}
	if !ismesvalid(text) {
		return "", nil, ebadmes
	}
	room, ok := sim.rooms[rid]
	if !ok {
		return "", nil, ebadroom
	}
	if name, ok := room[cid]; ok {
		return name, maps.Keys(room), nil
	}
	return "", nil, ebadroom
}

func (sim simulation) endconn(cid connid) (iter.Seq[uint32], error) {
	if _, ok := sim.online[cid]; !ok {
		return nil, errNotOnline
	}
	delete(sim.online, cid)
	return maps.Keys(sim.clirooms(cid)), nil
}

func (sim simulation) clirooms(cid connid) map[uint32]string {
	m := make(map[uint32]string)
	for rid, room := range sim.rooms {
		if name, ok := room[cid]; ok {
			m[rid] = name
		}
	}
	return m
}

func (sim simulation) lsro(cid connid) (string, error) {
	m := sim.clirooms(cid)
	rooms := slices.Sorted(maps.Keys(m))
	rows := make([]string, 0, len(rooms)+1)
	rows = append(rows, "room,name\n")
	for _, room := range rooms {
		name := m[room]
		row := fmt.Sprintf("%v,%v\n", room, name)
		rows = append(rows, row)
	}
	return strings.Join(rows, ""), nil
}

func (sim simulation) handle(m connmes) ([]connmes, bool) {
	// TODO: simulation needs to be updated after recent adjustments (lsro, rols)
	switch m.t {
	case mbegc:
		ok := sim.startconn(m.cid)
		if !ok {
			return nil, false
		}
	case mendc:
		var result []connmes
		rids, err := sim.endconn(m.cid)
		if err == nil {
			for rid := range rids {
				name, receivers, err := sim.exit(m.cid, rid)
				exed := exedmes(rid, name)
				if err == nil {
					result = addconnids(result, receivers, exed)
				}
			}
			return result, true
		}
	case mping:
		pong := pongmes()
		return []connmes{makeconnmes(m.cid, pong)}, true
	case mjoin:
		receivers, err := sim.join(m.cid, m.room, m.name.Value())
		if err == nil {
			jned := jnedmes(m.room, m.name.Value())
			return addconnids(nil, receivers, jned), true
		} else if e, ok := err.(ecode); ok {
			return []connmes{makeconnmes(m.cid, probmes(e))}, true
		}
	case mexit:
		name, receivers, err := sim.exit(m.cid, m.room)
		if err == nil {
			exed := exedmes(m.room, name)
			return addconnids(nil, receivers, exed), true
		} else if e, ok := err.(ecode); ok {
			return []connmes{makeconnmes(m.cid, probmes(e))}, true
		}
	case mtalk:
		text := m.text.Value()
		name, receivers, err := sim.talk(m.cid, text, m.room)
		if err == nil {
			hear := hearmes(m.room, name, text)
			return addconnids(nil, receivers, hear), true
		} else if e, ok := err.(ecode); ok {
			return []connmes{makeconnmes(m.cid, probmes(e))}, true
		}
	case mlsro:
		csv, err := sim.lsro(m.cid)
		if err == nil {
			rols := makeconnmes(m.cid, rolsmes(csv))
			return []connmes{rols}, true
		} else if e, ok := err.(ecode); ok {
			return []connmes{makeconnmes(m.cid, probmes(e))}, true
		}
	}
	return nil, false
}

func addconnids(cms []connmes, cids iter.Seq[connid], m protomes) []connmes {
	if cids == nil {
		return nil
	}
	for cid := range cids {
		cm := makeconnmes(cid, m)
		cms = append(cms, cm)
	}
	return cms
}

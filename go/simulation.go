package main

import (
	"fmt"
	"iter"
	"maps"
	"sync"
	"unique"
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

// connection start
// returns false if there was already a connection with the given id
func (sim simulation) startconn(cid connid) bool {
	if _, ok := sim.online[cid]; ok {
		return false
	}
	sim.online[cid] = zero{}
	return true
}

// returns simconnids of users who should receive the jned message
// also returns false if the user was already in the room, or the name is already in use; true otherwise
func (sim simulation) join(cid connid, rid uint32, name string) (iter.Seq[connid], bool) {
	if _, ok := sim.online[cid]; !ok {
		return nil, false
	}
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
	if _, ok := sim.online[cid]; !ok {
		return "", nil, false
	}
	room, ok := sim.rooms[rid]
	if !ok {
		return "", nil, false
	}
	if name, ok := room[cid]; ok {
		return name, maps.Keys(room), true
	}
	return "", nil, false
}

// connection ended
// returns rooms the connection had joined and under which name
// and false if there wasn't a connection with the given id in the first place
func (sim simulation) endconn(cid connid) (iter.Seq[uint32], bool) {
	if _, ok := sim.online[cid]; !ok {
		return nil, false
	}
	delete(sim.online, cid)
	it := func(yield func(uint32) bool) {
		for rid, room := range sim.rooms {
			if _, ok := room[cid]; ok {
				if !yield(rid) {
					break
				}
			}
		}
	}
	return it, true
}

func (sim simulation) handle(m connmes) ([]connmes, bool) {
	// TODO: if ok {} else {/* inconsistency! */}
	switch m.t {
	case mbegc:
		ok := sim.startconn(m.cid)
		if ok {
			return nil, false
		}
	case mendc:
		var result []connmes
		rids, ok := sim.endconn(m.cid)
		if ok {
			for rid := range rids {
				name, receivers, ok := sim.exit(m.cid, rid)
				exed := protomes{t: mexed, room: rid, name: name}
				if ok {
					result = addconnids(result, receivers, exed)
				}
			}
			return result, true
		}
	case mping:
		pong := protomes{t: mpong}
		return []connmes{makeconnmes(m.cid, pong)}, true
	case mjoin:
		receivers, ok := sim.join(m.cid, m.room, m.name.Value())
		if ok {
			jned := protomes{t: mjned, room: m.room, name: m.name.Value()}
			return addconnids(nil, receivers, jned), true
		}
	case mexit:
		name, receivers, ok := sim.exit(m.cid, m.room)
		if ok {
			exed := protomes{t: mexed, room: m.room, name: name}
			return addconnids(nil, receivers, exed), true
		}
	case mtalk:
		name, receivers, ok := sim.talk(m.cid, m.room)
		if ok {
			hear := protomes{t: mhear, room: m.room, name: name, text: m.text.Value()}
			return addconnids(nil, receivers, hear), true
		}
	}
	return nil, false
}

func addconnids(cms []connmes, cids iter.Seq[connid], m protomes) []connmes {
	for cid := range cids {
		cm := makeconnmes(cid, m)
		cms = append(cms, cm)
	}
	return cms
}

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
)

// id of a connection (user) in the simulation
type connid = int

// message along with an id of the connection from which it was sent OR which will receive it
type connmes struct {
	cid connid
	protomes
}

func (cm connmes) String() string {
	return fmt.Sprintf("{%d, %v}", cm.cid, cm.protomes)
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
	default:
		return nil, fmt.Errorf("not a call message: %v", m.t)
	}
	// (inconsistency!)
	return nil, nil
}

func addconnids(cids iter.Seq[connid], m protomes) []connmes {
	cms := make([]connmes, 0)
	for cid := range cids {
		cm := connmes{cid, m}
		cms = append(cms, cm)
	}
	return cms
}

type mesindex struct {
	idxs []int
	ms   []protomes
}

func (mi mesindex) after(idx int) []protomes {
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

	nextcid := 1
	connids := make(map[string]connid)

	connmi := make(map[connid]mesindex)
	cms := make([]connmes, len(lms))

	for i, lm := range lms {
		cid, ok := connids[lm.connName]
		if !ok {
			cid = nextcid
			nextcid++
			connids[lm.connName] = cid
		}
		cm := connmes{
			cid:      cid,
			protomes: lm.m,
		}
		cms[i] = cm

		mi := connmi[cid] // zero value usable
		connmi[cid] = mesindex{
			idxs: append(mi.idxs, i),
			ms:   append(mi.ms, cm.protomes),
		}
	}

	sim := makeSimulation()
	for i, cm := range cms {
		if !cm.t.iscall() {
			continue
		}
		call := cm
		casts, err := sim.handlecall(call)
		if err != nil {
			log.Fatal(err)
		}
		anymissing := false
		for _, cast := range casts {
			mi, ok := connmi[cast.cid]
			if !ok {
				log.Fatalf("mesindex not found for conn id %v", cast.cid)
			}
			if !slices.ContainsFunc(mi.after(i), cast.protomes.equal) {
				if !anymissing {
					anymissing = true
					fmt.Printf("\nmissing after %v:\n", call)
				}
				fmt.Printf("\t%v\n", cast)
			}
		}
	}

	// TODO: also detect casts that should NOT have been sent

}

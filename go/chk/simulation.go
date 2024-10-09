package chk

import (
	"fmt"
	"iter"
	"maps"
	"slices"
	"strings"
	"tccgo/mes"
)

var (
	ErrNotOnline = fmt.Errorf("no connection with this id has begun")
)

type zero = struct{}

type Simulation struct {
	online map[connId]zero
	rooms  map[uint32]map[connId]string
}

func MakeSimulation() Simulation {
	return Simulation{
		online: make(map[connId]zero),
		rooms:  make(map[uint32]map[connId]string),
	}
}

func (sim Simulation) startconn(cid connId) bool {
	if _, ok := sim.online[cid]; ok {
		return false
	}
	sim.online[cid] = zero{}
	return true
}

func (sim Simulation) join(cid connId, rid uint32, name string) (iter.Seq[connId], error) {
	if _, ok := sim.online[cid]; !ok {
		return nil, ErrNotOnline
	}
	if !mes.NameValid(name) {
		return nil, mes.ErrBadName
	}
	nrooms := len(sim.clirooms(cid))
	// TODO: ClientMaxRooms constant
	if nrooms >= 256 {
		return nil, mes.ErrRoomLimit
	}
	room, ok := sim.rooms[rid]
	if !ok {
		room = make(map[connId]string)
		sim.rooms[rid] = room
	}
	if _, ok := room[cid]; ok {
		return nil, mes.ErrJoined
	}
	// TODO: room capacity constant
	if len(room) >= 256 {
		return nil, mes.ErrRoomFull
	}
	for _, othername := range room {
		if othername == name {
			return nil, mes.ErrNameInUse
		}
	}
	room[cid] = name
	return maps.Keys(room), nil
}

func (sim Simulation) exit(cid connId, rid uint32) (string, iter.Seq[connId], error) {
	room, ok := sim.rooms[rid]
	if !ok {
		return "", nil, mes.ErrBadRoom
	}
	if name, ok := room[cid]; ok {
		delete(room, cid)
		return name, maps.Keys(room), nil
	} else {
		return "", nil, mes.ErrBadRoom
	}
}

func (sim Simulation) clirooms(cid connId) map[uint32]string {
	m := make(map[uint32]string)
	for rid, room := range sim.rooms {
		if name, ok := room[cid]; ok {
			m[rid] = name
		}
	}
	return m
}

func (sim Simulation) lsro(cid connId) (string, error) {
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

func (sim Simulation) endconn(cid connId) (iter.Seq[uint32], error) {
	if _, ok := sim.online[cid]; !ok {
		return nil, ErrNotOnline
	}
	delete(sim.online, cid)
	return maps.Keys(sim.clirooms(cid)), nil
}

func (sim Simulation) talk(cid connId, text string, rid uint32) (string, iter.Seq[connId], error) {
	if _, ok := sim.online[cid]; !ok {
		return "", nil, ErrNotOnline
	}
	if !mes.TextValid(text) {
		return "", nil, mes.ErrBadMessage
	}
	room, ok := sim.rooms[rid]
	if !ok {
		return "", nil, mes.ErrBadRoom
	}
	if name, ok := room[cid]; ok {
		return name, maps.Keys(room), nil
	}
	return "", nil, mes.ErrBadRoom
}

func (sim Simulation) Handle(m ConnMessage) ([]ConnMessage, bool) {
	// TODO: simulation needs to be updated after recent adjustments (lsro, rols)
	switch m.T {
	case mes.BegcType:
		ok := sim.startconn(m.ConnId)
		if !ok {
			return nil, false
		}
	case mes.EndcType:
		var result []ConnMessage
		rids, err := sim.endconn(m.ConnId)
		if err == nil {
			for rid := range rids {
				name, receivers, err := sim.exit(m.ConnId, rid)
				exed := mes.Exed(rid, name)
				if err == nil {
					result = addconnids(result, receivers, exed)
				}
			}
			return result, true
		}
	case mes.PingType:
		pong := mes.Pong()
		return []ConnMessage{MakeConnMessage(m.ConnId, pong)}, true
	case mes.JoinType:
		receivers, err := sim.join(m.ConnId, m.Room, m.Name.Value())
		if err == nil {
			jned := mes.Jned(m.Room, m.Name.Value())
			return addconnids(nil, receivers, jned), true
		} else if e, ok := err.(mes.Error); ok {
			return []ConnMessage{MakeConnMessage(m.ConnId, mes.Prob(e))}, true
		}
	case mes.ExitType:
		name, receivers, err := sim.exit(m.ConnId, m.Room)
		if err == nil {
			exed := mes.Exed(m.Room, name)
			return addconnids(nil, receivers, exed), true
		} else if e, ok := err.(mes.Error); ok {
			return []ConnMessage{MakeConnMessage(m.ConnId, mes.Prob(e))}, true
		}
	case mes.TalkType:
		text := m.Text.Value()
		name, receivers, err := sim.talk(m.ConnId, text, m.Room)
		if err == nil {
			hear := mes.Hear(m.Room, name, text)
			return addconnids(nil, receivers, hear), true
		} else if e, ok := err.(mes.Error); ok {
			return []ConnMessage{MakeConnMessage(m.ConnId, mes.Prob(e))}, true
		}
	case mes.LsroType:
		csv, err := sim.lsro(m.ConnId)
		if err == nil {
			rols := MakeConnMessage(m.ConnId, mes.Rols(csv))
			return []ConnMessage{rols}, true
		} else if e, ok := err.(mes.Error); ok {
			return []ConnMessage{MakeConnMessage(m.ConnId, mes.Prob(e))}, true
		}
	}
	return nil, false
}

func addconnids(cms []ConnMessage, cids iter.Seq[connId], m mes.Message) []ConnMessage {
	if cids == nil {
		return nil
	}
	for cid := range cids {
		cm := MakeConnMessage(cid, m)
		cms = append(cms, cm)
	}
	return cms
}

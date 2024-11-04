package main

import (
	"maps"
	"slices"
)

type connid = int
type topic = uint16

type simulation struct {
	mu    chan zero
	conns map[topic]map[connid]zero
}

func makeSimulation() simulation {
	return simulation{
		mu:    make(chan zero, 1),
		conns: make(map[topic]map[connid]zero),
	}
}

func (sim simulation) sub(cid connid, t topic) {
	sim.mu <- zero{}
	defer func() { <-sim.mu }()

	conns, ok := sim.conns[t]
	if !ok {
		conns = make(map[connid]zero)
		sim.conns[t] = conns
	}
	conns[cid] = zero{}
}

func (sim simulation) unsub(cid connid, t topic) {
	sim.mu <- zero{}
	defer func() { <-sim.mu }()

	if conns, ok := sim.conns[t]; ok {
		delete(conns, cid)
		if len(conns) == 0 {
			delete(sim.conns, t)
		}
	}
}

func (sim simulation) pub(t topic) []connid {
	sim.mu <- zero{}
	defer func() { <-sim.mu }()

	if conns, ok := sim.conns[t]; ok {
		return slices.Collect(maps.Keys(conns))
	}
	return nil
}

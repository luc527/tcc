package chk

import (
	"fmt"
	"iter"
	"slices"
	"sync"
	"tccgo/mes"
)

type miEntry struct {
	is []int
	ms []ConnMessage
}

// connid -> indexes into conn message list
type messageIndex map[int]miEntry

func (mi messageIndex) append(i int, cm ConnMessage) {
	cid := cm.ConnId
	entry, ok := mi[cid]
	if !ok {
		entry = miEntry{}
	}
	entry.is = append(entry.is, i)
	entry.ms = append(entry.ms, cm)
	mi[cid] = entry
}

// finds the index where cm is in the canonical connmessage list
// but only looks for messages with index > i
func (mi messageIndex) findIndexAfter(cm ConnMessage, i int) int {
	cid := cm.ConnId

	entry, ok := mi[cid]
	if !ok {
		return -1
	}

	from, _ := slices.BinarySearch(entry.is, i)
	after := entry.ms[from:]

	j := slices.IndexFunc(after, cm.equalmes)
	if j == -1 {
		return -1
	}
	j += from

	i_ := entry.is[j]
	return i_
}

type CheckerResults struct {
	List []ConnMessage
	Want [][]ConnMessage
	Got  [][]int // indexes into 'list'
}

type CheckerMissing struct {
	Sent ConnMessage
	Want ConnMessage
}

type Checker struct {
	mu    sync.Mutex
	sim   Simulation
	mi    messageIndex
	res   CheckerResults
	okres bool
}

func NewChecker() *Checker {
	c := &Checker{
		sim: MakeSimulation(),
		mi:  make(messageIndex),
	}
	return c
}

func (c *Checker) StartConn(id int) {
	c.Accept(id, mes.Message{T: mes.BegcType})
}

func (c *Checker) EndConn(id int) {
	c.Accept(id, mes.Message{T: mes.EndcType})
}

func (c *Checker) append(cm ConnMessage) {
	c.mu.Lock()
	defer c.mu.Unlock()
	i := len(c.res.List)
	c.res.List = append(c.res.List, cm)

	wants, _ := c.sim.Handle(cm)
	c.res.Want = append(c.res.Want, wants)
	c.res.Got = append(c.res.Got, make([]int, len(wants)))

	c.mi.append(i, cm)
}

func (c *Checker) Accept(connId int, m mes.Message) {
	cm := MakeConnMessage(connId, m)
	c.append(cm)
}

func (c *Checker) Results() CheckerResults {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.okres {
		return c.res
	}
	for i, wants := range c.res.Want {
		for j, want := range wants {
			c.res.Got[i][j] = c.mi.findIndexAfter(want, i)
		}
	}
	c.okres = true
	return c.res
}

func (cr CheckerResults) Missing() iter.Seq[CheckerMissing] {
	return func(yield func(CheckerMissing) bool) {
		for i, sent := range cr.List {
			for j, want := range cr.Want[i] {
				if got := cr.Got[i][j]; got == -1 {
					if !yield(CheckerMissing{sent, want}) {
						return
					}
				}
			}
		}
	}
}

func (cm CheckerMissing) String() string {
	return fmt.Sprintf("sent %v, wanted %v, got nothing", cm.Sent, cm.Want)
}

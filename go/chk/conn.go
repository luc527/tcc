package chk

import (
	"fmt"
	"sync"
	"tccgo/mes"
	"unique"
)

type connId int

type ConnMessage struct {
	ConnId connId
	T      mes.Type
	Room   uint32
	Name   unique.Handle[string]
	Text   unique.Handle[string]
}

type ConnIdProvider struct {
	mu sync.Mutex
	m  map[string]connId
}

func NewConnIdProvider() *ConnIdProvider {
	return &ConnIdProvider{
		sync.Mutex{},
		make(map[string]connId),
	}
}

func (cip *ConnIdProvider) ConnIdFor(s string) connId {
	cip.mu.Lock()
	defer cip.mu.Unlock()
	cid, ok := cip.m[s]
	if !ok {
		cid = connId(len(cip.m))
		cip.m[s] = cid
	}
	return cid
}

// only create ConnMessage through this function
// otherwise, you might leave name and text as the unique.Handle zero value, which contains a nil pointer which is dereferenced when you call .Value()
func MakeConnMessage(cid connId, m mes.Message) ConnMessage {
	return ConnMessage{
		ConnId: cid,
		T:      m.T,
		Room:   m.Room,
		Name:   unique.Make(m.Name),
		Text:   unique.Make(m.Text),
	}
}

func (cm ConnMessage) String() string {
	return fmt.Sprintf("{%d, %v}", cm.ConnId, mes.ToString(cm.T, cm.Room, cm.Name.Value(), cm.Text.Value()))
}

func (cm ConnMessage) message() mes.Message {
	return mes.Message{
		T:    cm.T,
		Room: cm.Room,
		Name: cm.Name.Value(),
		Text: cm.Name.Value(),
	}
}

func (this ConnMessage) equal(that ConnMessage) bool {
	if this.ConnId != that.ConnId {
		return false
	}
	return this.equalmes(that)
}

func (this ConnMessage) equalmes(that ConnMessage) bool {
	if this.T != that.T {
		return false
	}
	if this.T.HasRoom() && this.Room != that.Room {
		return false
	}
	if this.T.HasName() && this.Name != that.Name {
		return false
	}
	if this.T.HasText() && this.Text != that.Text {
		return false
	}
	return true
}

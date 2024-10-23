package main

import (
	"bytes"
	"io"
	"testing"
)

func TestProtocol(t *testing.T) {
	ms := []msg{
		{t: pingMsg},
		{t: pingMsg, topic: 123, payload: "abcba"},
		{t: subMsg, topic: 666},
		{t: subbedMsg, topic: 120, payload: "lol ignored", subbed: false},
		{t: subbedMsg, topic: 120, subbed: true},
		{t: subMsg, topic: 666, payload: "asdf"},
		{t: pubMsg, topic: 99, payload: "hello"},
		{t: pubMsg, topic: 129, payload: "now this is a really really long message :) üỳʔ oo--"},
		{t: pubMsg, topic: 0, payload: ""},
	}
	bb := new(bytes.Buffer)
	for _, m := range ms {
		bb.Reset()
		if n, err := m.WriteTo(bb); err != nil {
			t.Log(err)
			t.FailNow()
		} else {
			t.Logf("wrote %d", n)
		}
		var mm msg
		bs := bb.Bytes()
		if n, err := mm.ReadFrom(bytes.NewReader(bs)); err != nil && err != io.EOF {
			t.Log(bb.Bytes())
			t.Log(m)
			t.Log(mm)
			t.Log(err)
			t.FailNow()
		} else {
			t.Logf("read %d", n)
		}

		if !m.eq(mm) {
			t.Logf("wanted %v got %v", m, mm)
			t.FailNow()
		}
	}
}

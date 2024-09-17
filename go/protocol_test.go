package main

import (
	"bytes"
	"crypto/rand"
	"io"
	"math"
	"reflect"
	"slices"
	"testing"
)

// TODO: include mprob in tests

// TODO: test hashroom, hasname, hastext

// TODO: also test the size of the encoded messages, e.g. ping only 1 byte

func TestMtypes(t *testing.T) {
	all := []mtype{mping, mpong, mjoin, mjned, mtalk, mhear, mexit, mexed, mprob}

	for i := range all {
		for j := range all {
			if i != j && all[i] == all[j] {
				t.Logf("code conflict for %02x (indexes %d, %d)", all[i], i, j)
			}
		}
	}

	ornot := func(b bool) string {
		s := " "
		if !b {
			s = " NOT "
		}
		return s
	}

	for b := range math.MaxUint8 {
		mt := mtype(b)
		want := slices.Contains(all, mt)
		got := mt.valid()
		if want != got {
			t.Logf("expected 0x%02x to%sbe a valid mtype", mt, ornot(want))
		}
	}
}

// TODO: test decoding of invalid messages

func TestMessageEncodingAndDecodingValid(t *testing.T) {
	type testcase struct {
		m   protomes
		err error
	}

	var longok string
	{
		b := new(bytes.Buffer)
		io.Copy(b, io.LimitReader(rand.Reader, maxMessageLength))
		longok = b.String()
	}

	var longerr string
	{
		b := new(bytes.Buffer)
		io.Copy(b, io.LimitReader(rand.Reader, maxMessageLength+1))
		longerr = b.String()
	}

	bb := new(bytes.Buffer)

	testcases := []testcase{
		{protomes{t: mping}, nil},
		{protomes{t: mpong}, nil},
		{protomes{t: 0xff}, errInvalidMessageType},
		{protomes{t: mexit, room: 0}, nil},
		{protomes{t: mexit, room: 0xabcdef01}, nil},
		{protomes{t: mexit, room: math.MaxUint32}, nil},
		{protomes{t: mjoin, room: 2567, name: "helloo 1234!!!"}, nil},
		{protomes{t: mjoin, room: 2567, name: "this is a fairly long string"}, errNameTooLong},
		{protomes{t: mjoin, room: 2567, name: ""}, errNameEmpty},
		{protomes{t: mtalk, room: 9999, text: "hello friends"}, nil},
		{protomes{t: mtalk, room: 9999, text: ""}, nil},
		{protomes{t: mtalk, room: 9999, text: longok}, nil},
		{protomes{t: mtalk, room: 9999, text: longerr}, errMessageTooLong},
		{protomes{t: mhear, room: 7172, name: "figmund", text: "mi nombre es figmundo"}, nil},
	}

	for _, tc := range testcases {
		bb.Reset()
		_, err := tc.m.WriteTo(bb)
		if err != tc.err {
			t.Logf("expected err %v; got %v; for %v", tc.err, err, tc.m)
			t.Fail()
			continue
		}
		if err != nil {
			continue
		}
		m := protomes{}
		if _, err := m.ReadFrom(bb); err != nil {
			t.Logf("failed decoding %v", tc.m)
			t.Fail()
		}
		if !reflect.DeepEqual(tc.m, m) {
			t.Logf("message changed\nbefore: %v\nafter:  %v", tc.m, m)
			if tc.m.t == m.t && m.t.hastext() {
				t.Logf("text len before %d, after: %d, bb len: %d", len(tc.m.text), len(m.text), bb.Len())
			}
			t.Fail()
		}
	}
}

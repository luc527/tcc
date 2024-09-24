package main

import (
	"bytes"
	"fmt"
	"io"
	"strings"
)

// message type
type mtype uint8

const (
	// bit 0x10 means it's sent by the server, otherwise by the client
	// ^ is the pin operator, as in Elixir
	mping = mtype(0x00) // ^mping:1
	mpong = mtype(0x10) // ^mpong:1
	mtalk = mtype(0x01) // ^mtalk:1, room:4, textlen:2, text:^textlen
	mhear = mtype(0x11) // ^mhear:1, room:4, namelen:1, textlen:2, name:^namelen, text:^textlen
	mjoin = mtype(0x02) // ^mjoin:1, room:4, namelen:1, name:^namelen
	mjned = mtype(0x12) // ^mjned:1, room:4, namelen:1, name:^namelen
	mexit = mtype(0x04) // ^mexit:1, room:4
	mexed = mtype(0x14) // ^mexed:1, room:4, namelen:2, name:^namelen
	mlsro = mtype(0x08) // ^mlsro:1 (list rooms)
	mrols = mtype(0x18) // ^mrols:1, textlen:2, text:^textlen (room list)
	mprob = mtype(0xA0) // ^mprob:1, room:4
)

const (
	// begc and endc are not "real" message types, meaning they're not really sent by either client or server
	// they exists to signal when a connection has started or ended
	// which is necessary in order to run the simulation (simulation.go) correctly
	mbegc = mtype(0xF0)
	mendc = mtype(0xF1)
)

// error code -- will be sent as room in mprob message, only uses 2 out of 4 bytes though
type ecode uint32

const (
	ebadtype = ecode(mprob) << 8 // invalid message type

	// tried to join a room but...
	ejoined    = (ecode(mjoin) << 8) | 0x00 // you're already a member
	ebadname   = (ecode(mjoin) << 8) | 0x01 // name is empty or too long
	enameinuse = (ecode(mjoin) << 8) | 0x02 // someone is already using that name
	eroomlimit = (ecode(mjoin) << 8) | 0x03 // you've reached your limit and can't join any more rooms
	eroomfull  = (ecode(mjoin) << 8) | 0x04 // it's full

	// tried to send a message to a room but...
	ebadmes = (ecode(mtalk) << 8) | 0x00 // message is empty or too long

	// tried to either send a message to a room or exit a room but...
	ebadroom = (ecode(mtalk|mexit) << 8) | 0x00 // you haven't joined that room

	// a lot of errors are transient, due to timeouts, more specifically some "benign" data race
	// e.g. conn A sends "join room 10"
	// -> server sees that room exists, will send join request
	// -> but before that the only remaining user leaves, so the room gets removed from memory
	// -> the join request will timeout
	// I'm not sure this counts as a data race, but I'm saying it's "benign" because it doesn't really cause any problems (or I hope so)
	etransientsuffix = ecode(0xFF)
)

func etransient(t mtype) ecode {
	return (ecode(t) << 8) | etransientsuffix
}

var edescmap = map[ecode]string{
	ebadtype:   "bad message type",
	ejoined:    "you've already joined this room",
	ebadname:   "bad name, empty or too long",
	enameinuse: "name in use",
	eroomlimit: "you can't join any more rooms",
	eroomfull:  "this room is full",
	ebadmes:    "bad message, empty or too long",
	ebadroom:   "you haven't joined this room",
}

func (e ecode) String() string {
	s, ok := edescmap[e]
	if ok {
		return s
	}
	if (e & 0xFF) == etransientsuffix {
		return fmt.Sprintf("transient error for %s, try again", mtype(e>>8)&0xFF)
	}
	return fmt.Sprintf("0x%02x undefined error", uint32(e))
}

func (e ecode) Error() string {
	return e.String()
}

var _ fmt.Stringer = ebadtype
var _ error = ebadtype

// protocol message
type protomes struct {
	t    mtype
	room uint32
	name string
	text string
}

var _ io.WriterTo = &protomes{}
var _ io.ReaderFrom = &protomes{}
var _ fmt.Stringer = protomes{}

func (t mtype) valid() bool {
	return false ||
		t == mping || t == mpong ||
		t == mtalk || t == mhear ||
		t == mjoin || t == mjned ||
		t == mexit || t == mexed ||
		t == mlsro || t == mrols ||
		t == mprob

	// mbegc and mendc are not included here because
	// they shouldn't be sent by either client or server.
}

func (t mtype) hasroom() bool {
	return true &&
		t != mping && t != mpong &&
		t != mbegc && t != mendc &&
		t != mlsro && t != mrols
}

func (t mtype) hasname() bool {
	return t == mjoin || t == mjned || t == mexed || t == mhear
}

func (t mtype) hastext() bool {
	return t == mtalk || t == mhear || t == mrols
}

func (t mtype) String() string {
	switch t {
	case mjoin:
		return "join"
	case mexit:
		return "exit"
	case mtalk:
		return "talk"
	case mhear:
		return "hear"
	case mping:
		return "ping"
	case mpong:
		return "pong"
	case mjned:
		return "jned"
	case mexed:
		return "exed"
	case mprob:
		return "prob"
	case mbegc:
		return "begc"
	case mendc:
		return "endc"
	case mlsro:
		return "lsro"
	case mrols:
		return "rols"
	default:
		return ""
	}
}

func parseMtype(s string) (mtype, error) {
	switch s {
	case "join":
		return mjoin, nil
	case "exit":
		return mexit, nil
	case "talk":
		return mtalk, nil
	case "hear":
		return mhear, nil
	case "ping":
		return mping, nil
	case "pong":
		return mpong, nil
	case "jned":
		return mjned, nil
	case "exed":
		return mexed, nil
	case "prob":
		return mprob, nil
	case "begc":
		return mbegc, nil
	case "endc":
		return mendc, nil
	case "lsro":
		return mlsro, nil
	case "rols":
		return mrols, nil
	default:
		return 0, fmt.Errorf("invalid mtype string %q", s)
	}
}

func errormes(e ecode) protomes {
	return protomes{
		t:    mprob,
		room: uint32(e),
	}
}

func protomes2string(t mtype, room uint32, name string, text string) string {
	bb := new(bytes.Buffer)
	bb.WriteString("{")
	bb.WriteString(fmt.Sprintf("t: %q", t.String()))

	if t.hasroom() {
		bb.WriteString(", ")
		bb.WriteString(fmt.Sprintf("room: %d", room))
	}

	if t.hasname() {
		bb.WriteString(", name: ")
		bb.WriteRune('"')
		strings.NewReplacer(`"`, `\"`).WriteString(bb, name)
		bb.WriteRune('"')
	}
	if t.hastext() {
		bb.WriteString(", text: ")
		bb.WriteRune('"')
		strings.NewReplacer(`"`, `\"`).WriteString(bb, text)
		bb.WriteRune('"')
	}

	bb.WriteRune('}')

	return bb.String()
}

func (m protomes) String() string {
	return protomes2string(m.t, m.room, m.name, m.text)
}

// all numbers little endian

func (m protomes) WriteTo(w io.Writer) (n int64, err error) {
	if !m.t.valid() {
		return n, ebadtype
	}
	if nn, err := w.Write([]byte{byte(m.t)}); err != nil {
		return n, err
	} else {
		n += int64(nn)
	}

	if m.t.hasroom() {
		roombuf := []byte{
			byte(m.room),
			byte(m.room >> 8),
			byte(m.room >> 16),
			byte(m.room >> 24),
		}
		if nn, err := w.Write(roombuf); err != nil {
			return n, err
		} else {
			n += int64(nn)
		}
	}

	if m.t.hasname() {
		ln := len(m.name)
		if ln == 0 || ln > maxNameLength {
			return n, ebadname
		}
		if strings.Contains(m.name, "\n") {
			return n, ebadname
		}

		if nn, err := w.Write([]byte{byte(ln)}); err != nil {
			return n, err
		} else {
			n += int64(nn)
		}
	}

	if m.t.hastext() {
		ln := len(m.text)
		if ln == 0 || ln > maxMessageLength {
			return n, ebadmes
		}

		lnbuf := []byte{
			byte(ln),
			byte(ln >> 8),
		}
		if nn, err := w.Write(lnbuf); err != nil {
			return n, err
		} else {
			n += int64(nn)
		}
	}

	if m.t.hasname() {
		if nn, err := io.WriteString(w, m.name); err != nil {
			return n, err
		} else {
			n += int64(nn)
		}
	}

	if m.t.hastext() {
		if nn, err := io.WriteString(w, m.text); err != nil {
			return n, err
		} else {
			n += int64(nn)
		}
	}

	return n, nil
}

func (m *protomes) ReadFrom(r io.Reader) (n int64, err error) {
	tb := make([]byte, 1)
	if nn, err := readfull(r, tb); err != nil {
		return n, err
	} else {
		n += int64(nn)
	}
	t := mtype(tb[0])
	if !t.valid() {
		return n, ebadtype
	}
	m.t = t

	if m.t.hasroom() {
		rb := make([]byte, 4)
		if nn, err := readfull(r, rb); err != nil {
			return n, err
		} else {
			n += int64(nn)
		}
		room := uint32(rb[0]) | (uint32(rb[1]) << 8) | (uint32(rb[2]) << 16) | (uint32(rb[3]) << 24)
		m.room = room
	}

	var namelen uint8
	var textlen uint16

	if m.t.hasname() {
		lb := make([]byte, 1)
		if nn, err := readfull(r, lb); err != nil {
			return n, err
		} else {
			n += int64(nn)
		}
		namelen = uint8(lb[0])
	}

	if m.t.hastext() {
		lb := make([]byte, 2)
		if nn, err := readfull(r, lb); err != nil {
			return n, err
		} else {
			n += int64(nn)
		}
		textlen = uint16(lb[0]) | uint16(lb[1])<<8
	}

	if m.t.hasname() {
		bb := new(bytes.Buffer)
		bb.Grow(int(namelen))
		if nn, err := io.Copy(bb, io.LimitReader(r, int64(namelen))); err != nil {
			return n, err
		} else {
			n += int64(nn)
		}
		name := bb.String()
		m.name = name
	}

	if m.t.hastext() {
		bb := new(bytes.Buffer)
		bb.Grow(int(textlen))
		if nn, err := io.Copy(bb, io.LimitReader(r, int64(textlen))); err != nil {
			return n, err
		} else {
			n += int64(nn)
		}
		text := bb.String()
		m.text = text
	}

	return n, nil
}

func (this protomes) compare(that protomes) int64 {
	var c int64
	c = int64(this.t) - int64(that.t)
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

func (this protomes) equal(that protomes) bool {
	return this.compare(that) == 0
}

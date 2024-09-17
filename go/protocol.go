package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
)

// message type
type mtype byte

const (
	// bit 0x10 means it's sent by the server, otherwise by the client
	mping = mtype(0x00) // << ^mping:1 >>
	mpong = mtype(0x10) // << ^mpong:1 >>
	mtalk = mtype(0x01) // << ^mtalk:1, room:4, textlen:2, text:^textlen >>
	mhear = mtype(0x11) // << ^mhear:1, room:4, namelen:1, textlen:2, name:^namelen, text:^textlen >>
	mjoin = mtype(0x02) // << ^mjoin:1, room:4, namelen:1, name:^namelen >>
	mjned = mtype(0x12) // << ^mjned:1, room:4, namelen:1, name:^namelen >>
	mexit = mtype(0x04) // << ^mexit:1, room:4 >>
	mexed = mtype(0x14) // << ^mexed:1, room:4, namelen:2, name:^namelen >>
	mprob = mtype(0x20) // << ^mprob:1, room:4 >> (as in "problem")
	// ^ is the pin operator, as in Elixir
)

type protoerror struct {
	error
	code uint8
}

var _ error = protoerror{}

var (
	errInvalidMessageType = protoerror{errors.New("tccgo: message type is invalid"), 0x01}
	errMessageTooLong     = protoerror{errors.New("tccgo: message is too long"), 0x02}
	errNameTooLong        = protoerror{errors.New("tccgo: name is too long"), 0x04}
	errNameEmpty          = protoerror{errors.New("tccgo: name is empty"), 0x08}
	errJoinFailed         = protoerror{errors.New("tccgo: failed to join room; name might be in use"), 0x10}
)

var allprotoerrs = []protoerror{errInvalidMessageType, errMessageTooLong, errNameTooLong, errNameEmpty, errJoinFailed}

func protoerrcode(code uint8) (protoerror, bool) {
	for _, perr := range allprotoerrs {
		if perr.code == code {
			return perr, true
		}
	}
	return protoerror{}, false
}

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

const (
	maxMessageLength = 2048
	maxNameLength    = 24
)

func (t mtype) valid() bool {
	return t == mping || t == mpong ||
		t == mtalk || t == mhear ||
		t == mjoin || t == mjned ||
		t == mexit || t == mexed ||
		t == mprob
}

func (t mtype) hasroom() bool {
	return t != mping && t != mpong
}

func (t mtype) hasname() bool {
	return t == mjoin || t == mjned || t == mexed || t == mhear
}

func (t mtype) hastext() bool {
	return t == mtalk || t == mhear
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
	default:
		return "INVALIDMTYPE"
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
	default:
		return 0, fmt.Errorf("invalid mtype string %q", s)
	}
}

func errormes(err protoerror) protomes {
	return protomes{
		t:    mprob,
		room: uint32(err.code),
	}
}

func (m protomes) String() string {
	bb := new(bytes.Buffer)
	bb.WriteString("{")
	switch m.t {
	case mping:
		bb.WriteString("ping")
	case mpong:
		bb.WriteString("pong")
	case mtalk:
		bb.WriteString("talk")
	case mhear:
		bb.WriteString("hear")
	case mjoin:
		bb.WriteString("join")
	case mexit:
		bb.WriteString("exit")
	case mjned:
		bb.WriteString("jned")
	case mexed:
		bb.WriteString("exed")
	case mprob:
		bb.WriteString("prob")
	}

	if m.t.hasroom() {
		bb.WriteString(", ")
		bb.WriteString(fmt.Sprintf("%d", m.room))
	}

	if m.t.hasname() {
		bb.WriteString(", ")
		bb.WriteRune('"')
		strings.NewReplacer(`"`, `\"`).WriteString(bb, m.name)
		bb.WriteRune('"')
	}
	if m.t.hastext() {
		bb.WriteString(", ")
		bb.WriteRune('"')
		strings.NewReplacer(`"`, `\"`).WriteString(bb, m.text)
		bb.WriteRune('"')
	}

	bb.WriteRune('}')

	return bb.String()
}

// all numbers little endian

func (m protomes) WriteTo(w io.Writer) (n int64, err error) {
	if !m.t.valid() {
		return n, errInvalidMessageType
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
		if ln == 0 {
			return n, errNameEmpty
		} else if ln > maxNameLength {
			return n, errNameTooLong
		}

		if nn, err := w.Write([]byte{byte(ln)}); err != nil {
			return n, err
		} else {
			n += int64(nn)
		}
	}

	if m.t.hastext() {
		ln := len(m.text)
		if ln > maxMessageLength {
			return n, errMessageTooLong
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
		return n, errInvalidMessageType
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

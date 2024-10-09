package mes

import (
	"bytes"
	"fmt"
	"io"
	"strings"
)

const (
	MaxMessageLength = 24
	MaxNameLength    = 24
)

type Message struct {
	T    Type
	Room uint32
	Name string
	Text string
}

var _ io.WriterTo = &Message{}
var _ io.ReaderFrom = &Message{}
var _ fmt.Stringer = Message{}

func ToString(t Type, room uint32, name string, text string) string {
	bb := new(bytes.Buffer)
	bb.WriteString("{")
	bb.WriteString(fmt.Sprintf("t: %q", t.String()))

	if t.HasRoom() {
		bb.WriteString(", ")
		if t == ProbType {
			bb.WriteString(fmt.Sprintf("error: %q", Error(room)))
		} else {
			bb.WriteString(fmt.Sprintf("room: %d", room))
		}
	}

	if t.HasName() {
		bb.WriteString(", name: ")
		bb.WriteRune('"')
		strings.NewReplacer(`"`, `\"`).WriteString(bb, name)
		bb.WriteRune('"')
	}
	if t.HasText() {
		bb.WriteString(", text: ")
		bb.WriteRune('"')
		strings.NewReplacer(`"`, `\"`).WriteString(bb, text)
		bb.WriteRune('"')
	}

	bb.WriteRune('}')

	return bb.String()
}

func (m Message) String() string {
	return ToString(m.T, m.Room, m.Name, m.Text)
}

// all numbers little endian

func (m Message) WriteTo(w io.Writer) (n int64, err error) {
	if !m.T.Valid() {
		return n, ErrBadType
	}
	if nn, err := w.Write([]byte{byte(m.T)}); err != nil {
		return n, err
	} else {
		n += int64(nn)
	}

	if m.T.HasRoom() {
		roombuf := []byte{
			byte(m.Room),
			byte(m.Room >> 8),
			byte(m.Room >> 16),
			byte(m.Room >> 24),
		}
		if nn, err := w.Write(roombuf); err != nil {
			return n, err
		} else {
			n += int64(nn)
		}
	}

	if m.T.HasName() {
		ln := len(m.Name)
		if nn, err := w.Write([]byte{byte(ln)}); err != nil {
			return n, err
		} else {
			n += int64(nn)
		}
	}

	if m.T.HasText() {
		ln := len(m.Text)
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

	if m.T.HasName() {
		if nn, err := io.WriteString(w, m.Name); err != nil {
			return n, err
		} else {
			n += int64(nn)
		}
	}

	if m.T.HasText() {
		if nn, err := io.WriteString(w, m.Text); err != nil {
			return n, err
		} else {
			n += int64(nn)
		}
	}

	return n, nil
}

func (m *Message) ReadFrom(r io.Reader) (n int64, err error) {
	tb := make([]byte, 1)
	if nn, err := readfull(r, tb); err != nil {
		return n, err
	} else {
		n += int64(nn)
	}
	t := Type(tb[0])
	if !t.Valid() {
		return n, ErrBadType
	}
	m.T = t

	if m.T.HasRoom() {
		rb := make([]byte, 4)
		if nn, err := readfull(r, rb); err != nil {
			return n, err
		} else {
			n += int64(nn)
		}
		room := uint32(rb[0]) | (uint32(rb[1]) << 8) | (uint32(rb[2]) << 16) | (uint32(rb[3]) << 24)
		m.Room = room
	}

	var namelen uint8
	var textlen uint16

	if m.T.HasName() {
		lb := make([]byte, 1)
		if nn, err := readfull(r, lb); err != nil {
			return n, err
		} else {
			n += int64(nn)
		}
		namelen = uint8(lb[0])
	}

	if m.T.HasText() {
		lb := make([]byte, 2)
		if nn, err := readfull(r, lb); err != nil {
			return n, err
		} else {
			n += int64(nn)
		}
		textlen = uint16(lb[0]) | uint16(lb[1])<<8
	}

	if m.T.HasName() {
		bb := new(bytes.Buffer)
		bb.Grow(int(namelen))
		if nn, err := io.Copy(bb, io.LimitReader(r, int64(namelen))); err != nil {
			return n, err
		} else {
			n += int64(nn)
		}
		name := bb.String()
		m.Name = name
	}

	if m.T.HasText() {
		bb := new(bytes.Buffer)
		bb.Grow(int(textlen))
		if nn, err := io.Copy(bb, io.LimitReader(r, int64(textlen))); err != nil {
			return n, err
		} else {
			n += int64(nn)
		}
		text := bb.String()
		m.Text = text
	}

	return n, nil
}

func NameValid(s string) bool {
	n := len(s)
	if n == 0 || n > MaxNameLength || strings.Contains(s, "\n") {
		return false
	}
	return true
}

func TextValid(s string) bool {
	n := len(s)
	if n == 0 || n > MaxMessageLength {
		return false
	}
	return true
}

func Join(room uint32, name string) Message {
	return Message{T: JoinType, Room: room, Name: name}
}

func Jned(room uint32, name string) Message {
	return Message{T: JnedType, Room: room, Name: name}
}

func Exit(room uint32) Message {
	return Message{T: ExitType, Room: room}
}

func Exed(room uint32, name string) Message {
	return Message{T: ExedType, Room: room, Name: name}
}

func Talk(room uint32, text string) Message {
	return Message{T: TalkType, Room: room, Text: text}
}

func Hear(room uint32, name string, text string) Message {
	return Message{T: HearType, Room: room, Name: name, Text: text}
}

func Lsro() Message           { return Message{T: LsroType} }
func Rols(csv string) Message { return Message{T: RolsType, Text: csv} }
func Ping() Message           { return Message{T: PingType} }
func Pong() Message           { return Message{T: PongType} }

func Prob(e Error) Message {
	return Message{
		T:    ProbType,
		Room: uint32(e),
	}
}

func readfull(r io.Reader, destination []byte) (n int, err error) {
	remaining := destination
	for len(remaining) > 0 {
		if nn, err := r.Read(remaining); err != nil {
			return n, err
		} else {
			n += nn
			remaining = remaining[nn:]
		}
	}
	return n, err
}

func (m Message) Error() Error {
	return Error(m.Room)
}

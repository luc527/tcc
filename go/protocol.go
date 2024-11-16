package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

type msgType uint8

const (
	pingMsg  = msgType(0)
	pubMsg   = msgType(1)
	subMsg   = msgType(2)
	unsubMsg = msgType(4)
)

type msg struct {
	t       msgType
	topic   uint16
	payload string
}

var _ io.WriterTo = msg{}
var _ io.ReaderFrom = &msg{}
var _ fmt.Stringer = msg{}

// ping is a 0-sized msg

func (m msg) WriteTo(w io.Writer) (int64, error) {
	var (
		n   int64
		nn  int
		err error
		buf [2]byte
	)

	size := uint16(0)
	psize := uint16(len(m.payload))
	if m.t != pingMsg {
		size += 1 // type
		size += 2 // topic
		if m.t == pubMsg {
			size += psize
		}
	}

	binary.BigEndian.PutUint16(buf[:2], size)
	nn, err = writefull(w, buf[:2])
	if err != nil {
		return n, err
	}
	n += int64(nn)

	if m.t == pingMsg {
		return n, nil
	}

	buf[0] = byte(m.t)
	nn, err = writefull(w, buf[:1])
	if err != nil {
		return n, err
	}
	n += int64(nn)

	binary.BigEndian.PutUint16(buf[:2], m.topic)
	nn, err = writefull(w, buf[:2])
	if err != nil {
		return n, err
	}
	n += int64(nn)

	if m.t == pubMsg {
		nn, err = writefull(w, []byte(m.payload)[:int(psize)])
		if err != nil {
			return n, err
		}
		n += int64(nn)
	}

	return n, nil
}

func (m *msg) ReadFrom(r io.Reader) (int64, error) {
	var (
		n   int64
		nn  int
		err error
		buf [2]byte
	)

	nn, err = readfull(r, buf[:])
	if err != nil {
		return n, err
	}
	n += int64(nn)

	size := binary.BigEndian.Uint16(buf[:2])

	if size == 0 {
		m.t = pingMsg
		return n, nil
	}

	nn, err = readfull(r, buf[:1])
	if err != nil {
		return n, err
	}
	n += int64(nn)
	t := msgType(buf[0])
	m.t = t

	nn, err = readfull(r, buf[:2])
	if err != nil {
		return n, err
	}
	n += int64(nn)
	topic := binary.BigEndian.Uint16(buf[:2])
	m.topic = topic
	if err != nil {
		return n, err
	}

	if t == pubMsg {
		psize := size - 1 - 2 // - type - topic
		bb := new(bytes.Buffer)
		bb.Grow(int(psize))
		nn, err := bb.ReadFrom(io.LimitReader(r, int64(psize)))
		if err != nil {
			return n, err
		}
		n += int64(nn)

		payload := bb.String()
		m.payload = payload
	}

	return n, nil
}

func (a msg) eq(b msg) bool {
	if a.t != b.t {
		return false
	}
	if a.t != pingMsg {
		if a.topic != b.topic {
			return false
		}
		if a.t == pubMsg {
			if a.payload != b.payload {
				return false
			}
		}
	}
	return true
}

func (m msg) String() string {
	switch m.t {
	case pingMsg:
		return "msg{ping}"
	case subMsg:
		return fmt.Sprintf("msg{sub, %d}", m.topic)
	case unsubMsg:
		return fmt.Sprintf("msg{unsub, %d}", m.topic)
	case pubMsg:
		return fmt.Sprintf("msg{pub, %d, %q}", m.topic, m.payload)
	default:
		return fmt.Sprintf("msg{<invalid %d>, %d, %v}", m.t, m.topic, m.payload)
	}
}

func readfull(r io.Reader, buf []byte) (int, error) {
	n := 0
	rem := buf[:]
	for len(rem) > 0 {
		nn, err := r.Read(rem)
		if err != nil {
			return n, err
		}
		rem = rem[nn:]
		n += nn
	}
	return n, nil
}

func writefull(w io.Writer, buf []byte) (int, error) {
	n := 0
	rem := buf[:]
	for len(rem) > 0 {
		nn, err := w.Write(rem)
		if err != nil {
			return n, err
		}
		rem = rem[nn:]
		n += nn
	}
	return n, nil
}

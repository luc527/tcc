package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

type msgType uint8

const (
	pingMsg   = msgType(0x01)
	pubMsg    = msgType(0x02)
	subMsg    = msgType(0x03)
	unsubMsg  = msgType(0x04)
	subbedMsg = msgType(0x05)
)

type msg struct {
	t       msgType
	topic   uint16
	subbed  bool
	payload string
}

var _ io.WriterTo = msg{}
var _ io.ReaderFrom = &msg{}
var _ fmt.Stringer = msg{}

func (m msg) WriteTo(w io.Writer) (int64, error) {
	n := int64(0)
	buf := [2]byte{}

	var (
		nn  int
		err error
	)

	// write type (1 byte)
	buf[0] = byte(m.t)
	nn, err = writefull(w, buf[:1])
	if err != nil {
		return n, err
	}
	n += int64(nn)

	// write topic (2 bytes)
	if m.t == pubMsg || m.t == subMsg || m.t == unsubMsg || m.t == subbedMsg {
		binary.LittleEndian.PutUint16(buf[:], m.topic)
		nn, err = writefull(w, buf[:])
		if err != nil {
			return n, err
		}
		n += int64(nn)
	}

	// write subbed (1 byte)
	if m.t == subbedMsg {
		buf[0] = 0
		if m.subbed {
			buf[0] = 1
		}
		nn, err = writefull(w, buf[:1])
		if err != nil {
			return n, err
		}
		n += int64(nn)
	}

	// write payload
	if m.t == pubMsg {
		binary.LittleEndian.PutUint16(buf[:], uint16(len(m.payload)))
		nn, err = writefull(w, buf[:])
		if err != nil {
			return n, err
		}
		n += int64(nn)

		nn, err = writefull(w, []byte(m.payload))
		if err != nil {
			return n, err
		}
		n += int64(nn)
	}

	return n, nil
}

func (m *msg) ReadFrom(r io.Reader) (int64, error) {
	buf := [2]byte{}

	var (
		n   int64
		nn  int
		err error
	)

	nn, err = readfull(r, buf[:1])
	t := msgType(buf[0])
	m.t = t
	n += int64(nn)
	if err != nil {
		return n, err
	}

	if t == pubMsg || t == subMsg || t == unsubMsg || t == subbedMsg {
		nn, err = readfull(r, buf[:2])
		n += int64(nn)
		topic := binary.LittleEndian.Uint16(buf[:])
		m.topic = topic
		if err != nil {
			return n, err
		}
	}

	if t == subbedMsg {
		nn, err = readfull(r, buf[:1])
		n += int64(nn)
		m.subbed = buf[0] > 0
		if err != nil {
			return n, err
		}
	}

	if t == pubMsg {
		nn, err = readfull(r, buf[:2])
		n += int64(nn)
		if err != nil {
			return n, err
		}

		length := int(binary.LittleEndian.Uint16(buf[:]))
		bb := new(bytes.Buffer)
		bb.Grow(length)
		nn, err := bb.ReadFrom(io.LimitReader(r, int64(length)))
		n += int64(nn)
		payload := bb.String()
		m.payload = payload
		if err != nil {
			return n, err
		}
	}

	return n, nil
}

func (a msg) eq(b msg) bool {
	if a.t != b.t {
		return false
	}
	if a.t == pubMsg || a.t == subMsg || a.t == unsubMsg || a.t == subbedMsg {
		if a.topic != b.topic {
			return false
		}
	}
	if a.t == subbedMsg {
		if a.subbed != b.subbed {
			return false
		}
	}
	if a.t == pubMsg {
		if a.payload != b.payload {
			return false
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
	case pubMsg:
		return fmt.Sprintf("msg{pub, %d, %q}", m.topic, m.payload)
	case subbedMsg:
		return fmt.Sprintf("msg{subbed, %d, %v}", m.topic, m.subbed)
	default:
		return fmt.Sprintf("msg{<invalid %d>, %d, %v, %v}", m.t, m.topic, m.subbed, m.payload)
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

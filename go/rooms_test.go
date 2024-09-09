package main

import (
	"fmt"
	"testing"
	"time"
)

func discardchan[T any]() chan T {
	c := make(chan T)
	go func() {
		for range c {
		}
	}()
	return c
}

const (
	testReceiveTimeout = 10 * time.Millisecond
)

func assertClosed[T any](t *testing.T, id string, c <-chan T) {
	duration := testReceiveTimeout
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		t.Logf("%s: failed to receive close in %v", id, duration)
		t.FailNow()
	case _, ok := <-c:
		if ok {
			t.Logf("%s: failed to receive close, channel still open", id)
			t.FailNow()
		}
	}
}

func assertReceives[T comparable](t *testing.T, id string, c <-chan T, v T) {
	duration := testReceiveTimeout
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		t.Logf("%s: failed to receive %v in %v", id, v, duration)
		t.FailNow()
	case w, ok := <-c:
		if !ok {
			t.Logf("%s: failed to receive %v, channel closed", id, v)
			t.FailNow()
		}
		if w != v {
			t.Logf("%s: failed to receive %v, got %v", id, v, w)
			t.FailNow()
		}
	}
}

func assertNotReceives[T any](t *testing.T, id string, c <-chan T) {
	duration := testReceiveTimeout
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
	case _, ok := <-c:
		if ok {
			t.Logf("%s: failed to not receive", id)
			t.FailNow()
		}
	}
}

func assertJoinsRoom(t *testing.T, id string, prob <-chan zero, resp <-chan rhandle) rhandle {
	select {
	case <-prob:
		t.Logf("%s: failed to join", id)
		t.FailNow()
		return rhandle{}
	case handle := <-resp:
		return handle
	}
}

func assertCantJoinRoom(t *testing.T, id string, prob <-chan zero, resp <-chan rhandle) {
	select {
	case <-prob:
		return
	case <-resp:
		t.Logf("%v: should not have been able to join", id)
		t.FailNow()
	}
}

func assertRoomOpen(t *testing.T, id string, c <-chan zero) {
	assertNotReceives(t, fmt.Sprintf("%v: room should be open", id), c)
}

func assertRoomClosed(t *testing.T, id string, c <-chan zero) {
	assertClosed(t, fmt.Sprintf("%v: room should be closed", id), c)
}

func assertSends(t *testing.T, id string, c chan string, s string, x <-chan zero) {
	select {
	case c <- s:
	case <-x:
		t.Logf("%v: failed to send %q", id, s)
		t.FailNow()
	}
}

func TestJoinExit(t *testing.T) {
	r := makeroom()
	go r.run(0, discardchan[uint32]())

	prob := make(chan zero)
	resp := make(chan rhandle)

	jned := make(chan string)
	exed := make(chan string)
	cli := rclient{
		name: "fulano",
		out:  discardchan[rmes](),
		jned: jned,
		exed: exed,
	}

	r.joinreq <- rjoinreq{cli, prob, resp}
	handle := assertJoinsRoom(t, "fulano", prob, resp)
	assertRoomOpen(t, "fulano", handle.closed)
	assertReceives(t, "fulano", jned, "fulano")

	r.joinreq <- rjoinreq{cli, prob, resp}
	assertCantJoinRoom(t, "fulano", prob, resp)
	assertRoomOpen(t, "hfulano", handle.closed)

	close(handle.in)
	r.close()
	<-r.closed
	assertNotReceives(t, "fulano", exed)
	assertRoomClosed(t, "hfulano", handle.closed)
}

func TestRoom(t *testing.T) {
	r := makeroom()
	go r.run(0, discardchan[uint32]())

	prob := make(chan zero)
	resp := make(chan rhandle)

	// TODO: both clients (joao, jose) need to run in different goroutines
	// test currently failing because of that

	joaoOut := make(chan rmes)
	joaoJned := make(chan string)
	joaoExed := make(chan string)
	joaoCli := rclient{"joao", joaoOut, joaoJned, joaoExed}

	r.joinreq <- rjoinreq{joaoCli, prob, resp}
	joaoHandle := assertJoinsRoom(t, "joao", prob, resp)
	assertReceives(t, "joao received jned", joaoJned, "joao")

	var msg string

	msg = "iae meus manos"
	assertSends(t, "joao send 1", joaoHandle.in, msg, joaoHandle.closed)
	assertReceives(t, "joao receive 1", joaoOut, rmes{"joao", msg})

	joseOut := make(chan rmes)
	joseJned := make(chan string)
	joseExed := make(chan string)
	joseCli := rclient{"jose", joseOut, joseJned, joseExed}

	r.joinreq <- rjoinreq{joseCli, prob, resp}
	joseHandle := assertJoinsRoom(t, "jose", prob, resp)

	assertReceives(t, "joao received jose jned", joaoJned, "jose")
	assertReceives(t, "jose received jose jned", joseJned, "jose")

	msg = "abcd 1234"
	assertSends(t, "jose sends 2", joseHandle.in, msg, joseHandle.closed)
	assertReceives(t, "joao receives 2", joaoOut, rmes{"jose", msg})
	assertReceives(t, "jose receives 2", joseOut, rmes{"jose", msg})

	joaoHandle.close()
	assertClosed(t, "joao out closed", joaoOut)
	assertClosed(t, "joao jned closed", joaoJned)
	assertClosed(t, "joao exed closed", joaoExed)

	assertReceives(t, "jose receives joao exed", joseExed, "joao")

	r.close()
	<-r.closed

	assertClosed(t, "jose out closed", joseOut)
	assertClosed(t, "jose jned closed", joseJned)
	assertClosed(t, "jose exed closed", joseExed)
	joseHandle.close()
}

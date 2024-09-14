package main

import (
	"context"
	"testing"
	"time"
)

const roomTestTimeout = 10 * time.Millisecond

func assertReceives[T comparable](t *testing.T, s string, c chan T, v T) {
	timer := time.NewTimer(roomTestTimeout)
	defer timer.Stop()
	select {
	case <-timer.C:
		t.Logf("failed (%v): timeout %v", s, roomTestTimeout)
		t.FailNow()
	case w := <-c:
		if w != v {
			t.Logf("failed (%v): received %v, expected %v", s, w, v)
			t.FailNow()
		}
	}
}

func assertClosed[T any](t *testing.T, s string, c <-chan T) {
	timer := time.NewTimer(roomTestTimeout)
	defer timer.Stop()
	select {
	case <-timer.C:
		t.Logf("failed (%v): timeout %v", s, roomTestTimeout)
		t.FailNow()
	case v, ok := <-c:
		if ok {
			t.Logf("failed (%v): still open, received %v", s, v)
			t.FailNow()
		}
	}
}

func assertJoins(t *testing.T, s string, r room, name string, mesc chan roommes, jnedc chan string, exedc chan string) roomhandle {
	var rh roomhandle
	prob, resp := make(chan zero), make(chan roomhandle)
	req := joinroomreq{name, mesc, jnedc, exedc, resp, prob}
	r.join(req)
	select {
	case <-prob:
		t.Logf("failed (%v): failed to join (name %v)", s, name)
		t.FailNow()
	case rh = <-resp:
	}
	return rh
}

func assertSends[T any](t *testing.T, s string, c chan<- T, v T) {
	timer := time.NewTimer(roomTestTimeout)
	defer timer.Stop()
	select {
	case <-timer.C:
		t.Logf("failed (%v): timeout %v", s, roomTestTimeout)
		t.FailNow()
	case c <- v:
	}
}

// TODO: test trying to join after room closes
// TODO: test trying to join with the same name as someone else

func TestTwoJoinExit(t *testing.T) {
	msg := ""
	g := newgroup()

	r := startroom(0, makectx(context.Background()), make(chan uint32))

	joao := makeroomclient()
	maria := makeroomclient()

	g.run(func() { assertReceives(t, "joao <- joao jned", joao.jnedc, "joao") })
	joaoRh := assertJoins(t, "joins", r, "joao", joao.mesc, joao.jnedc, joao.exedc)
	g.wait()

	g.run(func() { assertReceives(t, "maria <- maria jned", maria.jnedc, "maria") })
	g.run(func() { assertReceives(t, "joao <- maria jned", joao.jnedc, "maria") })
	mariaRh := assertJoins(t, "joins", r, "maria", maria.mesc, maria.jnedc, maria.exedc)
	g.wait()

	msg = "hii"
	g.run(func() { assertReceives(t, "joao <- maria message", joao.mesc, roommes{"maria", msg}) })
	g.run(func() { assertReceives(t, "maria <- maria message", maria.mesc, roommes{"maria", msg}) })
	assertSends(t, "maria sends hi", mariaRh.mesc, msg)
	g.wait()

	msg = "hii HII"
	g.run(func() { assertReceives(t, "joao <- maria 2nd message", joao.mesc, roommes{"maria", msg}) })
	g.run(func() { assertReceives(t, "maria <- maria 2nd message", maria.mesc, roommes{"maria", msg}) })
	assertSends(t, "maria sends hi again", mariaRh.mesc, msg)
	g.wait()

	msg = "Good bye"
	g.run(func() { assertReceives(t, "joao <- joao message", joao.mesc, roommes{"joao", msg}) })
	g.run(func() { assertReceives(t, "maria <- joao message", maria.mesc, roommes{"joao", msg}) })
	assertSends(t, "joao sends bye", joaoRh.mesc, msg)
	g.wait()

	g.run(func() { assertReceives(t, "maria <- joao exited", maria.exedc, "joao") })
	joaoRh.cancel()
	assertClosed(t, "joao exited", joaoRh.done())
	g.wait()

	mariaRh.cancel()
	assertClosed(t, "maria exited", mariaRh.done())
	assertClosed(t, "room closed after maria exited", mariaRh.done())
}

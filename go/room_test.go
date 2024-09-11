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
	g := newg()

	r := startroom(makectx(context.Background()), 0, make(chan uint32))

	joaoMesc, joaoJnedc, joaoExedc := makeclientc()
	mariaMesc, mariaJnedc, mariaExedc := makeclientc()

	g.r(func() { assertReceives(t, "joao <- joao jned", joaoJnedc, "joao") })
	joaoRh := assertJoins(t, "joins", r, "joao", joaoMesc, joaoJnedc, joaoExedc)
	g.w()

	g.r(func() { assertReceives(t, "maria <- maria jned", mariaJnedc, "maria") })
	g.r(func() { assertReceives(t, "joao <- maria jned", joaoJnedc, "maria") })
	mariaRh := assertJoins(t, "joins", r, "maria", mariaMesc, mariaJnedc, mariaExedc)
	g.w()

	msg = "hii"
	g.r(func() { assertReceives(t, "joao <- maria message", joaoMesc, roommes{"maria", msg}) })
	g.r(func() { assertReceives(t, "maria <- maria message", mariaMesc, roommes{"maria", msg}) })
	assertSends(t, "maria sends hi", mariaRh.mesc, msg)
	g.w()

	msg = "hii HII"
	g.r(func() { assertReceives(t, "joao <- maria 2nd message", joaoMesc, roommes{"maria", msg}) })
	g.r(func() { assertReceives(t, "maria <- maria 2nd message", mariaMesc, roommes{"maria", msg}) })
	assertSends(t, "maria sends hi again", mariaRh.mesc, msg)
	g.w()

	msg = "Good bye"
	g.r(func() { assertReceives(t, "joao <- joao message", joaoMesc, roommes{"joao", msg}) })
	g.r(func() { assertReceives(t, "maria <- joao message", mariaMesc, roommes{"joao", msg}) })
	assertSends(t, "joao sends bye", joaoRh.mesc, msg)
	g.w()

	g.r(func() { assertReceives(t, "maria <- joao exited", mariaExedc, "joao") })
	joaoRh.cancel()
	assertClosed(t, "joao exited", joaoRh.done())
	g.w()

	mariaRh.cancel()
	assertClosed(t, "maria exited", mariaRh.done())
	assertClosed(t, "room closed after maria exited", mariaRh.done())
}

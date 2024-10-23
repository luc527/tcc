package main

import (
	"testing"
	"time"
)

func receiveWithin[T any](c <-chan T, t time.Duration) (T, bool) {
	var z T
	select {
	case <-time.After(t):
		return z, false
	case v := <-c:
		return v, true
	}
}

func assertReceives[T comparable](desc string, t *testing.T, c <-chan T, want T) {
	if got, ok := receiveWithin(c, 1*time.Second); ok {
		if want != got {
			t.Logf("%s: wanted %v, got %v", desc, want, got)
			t.FailNow()
		}
	} else {
		t.Logf("%s: did not receive", desc)
		t.FailNow()
	}
}

func TestServer(t *testing.T) {
	sp := newServerPartition(0)
	sp.start()
	defer sp.stop()

	var (
		theTopic   uint16
		thePayload string
	)

	theTopic = 7
	thePayload = "hello world"

	sub0 := makeSubscriber()
	if !sp.sub(theTopic, sub0) {
		t.Logf("sub0 failed to subscribe to %d", theTopic)
		t.FailNow()
	}
	assertReceives("sub0 subbed", t, sub0.subbedc, subbed{theTopic, true})

	if !sp.pub(theTopic, thePayload) {
		t.Logf("failed 1st publish")
		t.FailNow()
	}
	assertReceives("sub0 1st pub", t, sub0.pubc, publication{theTopic, thePayload})

	sub1 := makeSubscriber()
	if !sp.sub(theTopic, sub1) {
		t.Logf("sub1 failed to subscribe")
		t.FailNow()
	}
	assertReceives("sub1 subbed", t, sub1.subbedc, subbed{theTopic, true})

	thePayload = "what is this"

	if !sp.pub(theTopic, thePayload) {
		t.Logf("failed 2nd publish")
		t.FailNow()
	}

	assertReceives("sub0 2nd pub", t, sub0.pubc, publication{theTopic, thePayload})
	assertReceives("sub1 2nd pub", t, sub1.pubc, publication{theTopic, thePayload})

	if !sp.unsub(theTopic, sub0) {
		t.Logf("sub0 failed to unsubscribe")
		t.FailNow()
	}

	assertReceives("sub0 unsub", t, sub0.subbedc, subbed{theTopic, false})

	thePayload = "hello again"

	if !sp.pub(theTopic, thePayload) {
		t.Logf("failed 3rd publish")
		t.FailNow()
	}

	assertReceives("sub1 3rd pub", t, sub1.pubc, publication{theTopic, thePayload})
}

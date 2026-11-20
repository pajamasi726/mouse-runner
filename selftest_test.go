package main

import (
	"io"
	"testing"
	"time"
)

func advancing(f *fakeInput, typing bool, slept *time.Duration) func(time.Duration) {
	return func(d time.Duration) {
		*f.now = f.now.Add(d)
		*slept += d
		if typing {
			f.last = *f.now
		}
	}
}

func TestSelfTestPassesWhenNudgeResetsIdle(t *testing.T) {
	f := newFake(at(25, 10, 0))
	var slept time.Duration
	if !SelfTest(f, io.Discard, advancing(f, false, &slept)) || len(f.nudges) != 1 {
		t.Fatalf("want pass with one nudge, got %d nudges", len(f.nudges))
	}
}

func TestSelfTestFailsWhenNudgeIsIgnored(t *testing.T) {
	f := newFake(at(25, 10, 0))
	f.ignore = true
	var slept time.Duration
	if SelfTest(f, io.Discard, advancing(f, false, &slept)) {
		t.Fatal("want failure when the nudge does not reset idle time")
	}
}

func TestSelfTestGivesUpWhenUserKeepsTyping(t *testing.T) {
	f := newFake(at(25, 10, 0))
	var slept time.Duration
	if SelfTest(f, io.Discard, advancing(f, true, &slept)) || len(f.nudges) != 0 || slept > 31*time.Second {
		t.Fatalf("want give-up without nudging; nudges=%d slept=%s", len(f.nudges), slept)
	}
}

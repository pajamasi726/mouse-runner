package main

import (
	"errors"
	"flag"
	"io"
	"strings"
	"testing"
	"time"
)

func TestConfigDefaults(t *testing.T) {
	c, warn, err := ParseConfig(nil, io.Discard)
	if err != nil || len(warn) != 0 {
		t.Fatalf("err=%v warn=%v", err, warn)
	}
	if c.Min != time.Minute || c.Max != 8*time.Minute || c.Sched.String() != "항상" || c.SelfTest || c.Version {
		t.Fatalf("defaults = %+v", c)
	}
}

func TestConfigCustom(t *testing.T) {
	c, _, err := ParseConfig([]string{"-min", "30s", "-max", "4m", "-hours", "09:00-18:00", "-days", "mon-fri"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if c.Min != 30*time.Second || c.Max != 4*time.Minute {
		t.Fatalf("range = %s..%s", c.Min, c.Max)
	}
	expectActive(t, c.Sched, at(23, 10, 0), true)
	expectActive(t, c.Sched, at(21, 10, 0), false)
}

func TestConfigRejectsBadValues(t *testing.T) {
	for _, args := range [][]string{
		{"-min", "0s"}, {"-min", "5m", "-max", "2m"}, {"-hours", "9-18"}, {"-days", "monday"}, {"-max", "abc"},
		{"mon-fri"}, // -days를 빠뜨린 인자가 조용히 "항상"으로 돌면 안 됨
	} {
		if _, _, err := ParseConfig(args, io.Discard); err == nil {
			t.Errorf("ParseConfig(%q): want error", args)
		}
	}
}

func TestConfigWarnsOnLongMax(t *testing.T) {
	_, warn, err := ParseConfig([]string{"-max", "12m"}, io.Discard)
	if err != nil || len(warn) != 1 || !strings.Contains(warn[0], "10분") {
		t.Fatalf("err=%v warn=%v; want one warning mentioning 10분", err, warn)
	}
}

func TestConfigHelp(t *testing.T) {
	if _, _, err := ParseConfig([]string{"-h"}, io.Discard); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("err = %v, want flag.ErrHelp", err)
	}
}

package main

import (
	"strings"
	"testing"
)

func TestEnsureAccessOK(t *testing.T) {
	var out strings.Builder
	requested := 0
	ok := ensureAccess(func() bool { return true }, func() bool { requested++; return true }, &out)
	if !ok || requested != 0 || out.Len() != 0 {
		t.Fatalf("ok=%v requested=%d out=%q", ok, requested, out.String())
	}
}

func TestEnsureAccessExplainsMissingPermission(t *testing.T) {
	var out strings.Builder
	requested := 0
	ok := ensureAccess(func() bool { return false }, func() bool { requested++; return false }, &out)
	if ok || requested != 1 || !strings.Contains(out.String(), "손쉬운 사용") {
		t.Fatalf("ok=%v requested=%d out=%q", ok, requested, out.String())
	}
}

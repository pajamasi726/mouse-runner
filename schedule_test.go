package main

import (
	"testing"
	"time"
)

// 2026-11: 20=금 21=토 22=일 23=월 24=화 25=수
func at(day, hh, mm int) time.Time {
	return time.Date(2026, time.November, day, hh, mm, 0, 0, time.Local)
}

func mustSchedule(t *testing.T, hours, days string) Schedule {
	t.Helper()
	s, err := ParseSchedule(hours, days)
	if err != nil {
		t.Fatalf("ParseSchedule(%q, %q): %v", hours, days, err)
	}
	return s
}

func expectActive(t *testing.T, s Schedule, tm time.Time, want bool) {
	t.Helper()
	if got := s.Active(tm); got != want {
		t.Errorf("%s Active(%s) = %v, want %v", s, tm.Format("Mon 15:04"), got, want)
	}
}

func TestEmptyScheduleIsAlways(t *testing.T) {
	s := mustSchedule(t, "", "")
	expectActive(t, s, at(22, 3, 0), true)
	expectActive(t, s, at(25, 12, 0), true)
	if s.String() != "항상" {
		t.Errorf("String() = %q, want 항상", s.String())
	}
}

func TestWorkHours(t *testing.T) {
	s := mustSchedule(t, "09:00-18:00", "mon-fri")
	expectActive(t, s, at(23, 9, 0), true)
	expectActive(t, s, at(23, 8, 59), false)
	expectActive(t, s, at(23, 17, 59), true)
	expectActive(t, s, at(23, 18, 0), false)
	expectActive(t, s, at(21, 10, 0), false) // 토요일
	if s.String() != "mon-fri 09:00-18:00" {
		t.Errorf("String() = %q", s.String())
	}
}

func TestOvernightHours(t *testing.T) {
	s := mustSchedule(t, "22:00-06:00", "")
	expectActive(t, s, at(25, 23, 0), true)
	expectActive(t, s, at(25, 5, 59), true)
	expectActive(t, s, at(25, 6, 0), false)
	expectActive(t, s, at(25, 12, 0), false)
}

func TestEndOfDay(t *testing.T) {
	s := mustSchedule(t, "18:00-24:00", "")
	expectActive(t, s, at(25, 23, 59), true)
	expectActive(t, s, at(25, 17, 59), false)
}

func TestDayListsAndWrap(t *testing.T) {
	list := mustSchedule(t, "", "mon,wed,fri")
	expectActive(t, list, at(25, 12, 0), true)  // 수
	expectActive(t, list, at(24, 12, 0), false) // 화
	wrap := mustSchedule(t, "", "fri-mon")
	for _, d := range []int{20, 21, 22, 23} {
		expectActive(t, wrap, at(d, 12, 0), true)
	}
	expectActive(t, wrap, at(25, 12, 0), false)
	expectActive(t, mustSchedule(t, "", "SAT-sun"), at(21, 12, 0), true)
}

func TestScheduleErrors(t *testing.T) {
	for _, c := range []struct{ hours, days string }{
		{"9-18", ""}, {"25:00-26:00", ""}, {"09:00-09:00", ""}, {"09:00", ""}, {"09:60-10:00", ""},
		{"", "monday"}, {"", "mon-"}, {"", "xyz"}, {"", "mon,,fri"},
	} {
		if _, err := ParseSchedule(c.hours, c.days); err == nil {
			t.Errorf("ParseSchedule(%q, %q): want error", c.hours, c.days)
		}
	}
}

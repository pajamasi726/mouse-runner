package main

import (
	"math/rand/v2"
	"testing"
	"time"
)

func newTestRunner(f *fakeInput, s Schedule) *Runner {
	return &Runner{In: f, Sched: s, Min: time.Minute, Max: 8 * time.Minute,
		Rand: rand.New(rand.NewPCG(1, 2)), Logf: func(string, ...any) {}}
}

func runDay(t *testing.T) (f *fakeInput, maxIdle, maxSleep time.Duration) {
	t.Helper()
	start := at(25, 0, 0)
	f = newFake(start)
	maxIdle, maxSleep = simulate(newTestRunner(f, Schedule{}), f, start, 24*time.Hour, nil)
	return f, maxIdle, maxSleep
}

func TestIdleNeverExceedsMaxAndIntervalsAreRandom(t *testing.T) {
	f, maxIdle, _ := runDay(t)
	if maxIdle > 8*time.Minute {
		t.Fatalf("max idle %s > 8m", maxIdle)
	}
	if len(f.nudges) < 180 {
		t.Fatalf("only %d nudges in 24h, want >= 180", len(f.nudges))
	}
	distinct := map[time.Duration]bool{}
	for _, d := range f.idleAtNudge {
		if d < time.Minute || d > 8*time.Minute {
			t.Fatalf("nudge at idle %s, want within [1m, 8m]", d)
		}
		distinct[d] = true
	}
	if len(distinct) < 50 {
		t.Fatalf("only %d distinct intervals; timing looks periodic", len(distinct))
	}
}

func TestStepNeverSleepsLongerThanPollCap(t *testing.T) {
	if _, _, maxSleep := runDay(t); maxSleep > PollCap {
		t.Fatalf("max sleep %s > PollCap", maxSleep)
	}
}

func TestRecoversAfterSystemSleep(t *testing.T) {
	t0 := at(25, 10, 0)
	f := newFake(t0)
	r := newTestRunner(f, Schedule{})
	if d := r.Step(t0); d > PollCap {
		t.Fatalf("first Step slept %s", d)
	}
	*f.now = t0.Add(2 * time.Hour) // 2시간 잠들었다 깨어남, 그동안 입력 없음
	r.Step(*f.now)
	if len(f.nudges) != 1 || !f.nudges[0].Equal(*f.now) {
		t.Fatalf("nudges = %v, want one nudge right after wake", f.nudges)
	}
}

func TestNoNudgeWhileUserActive(t *testing.T) {
	start := at(25, 10, 0)
	f := newFake(start)
	simulate(newTestRunner(f, Schedule{}), f, start, 2*time.Hour, func(time.Time) bool { return true })
	if len(f.nudges) != 0 {
		t.Fatalf("got %d nudges while the user was active", len(f.nudges))
	}
}

func TestNoNudgeOutsideSchedule(t *testing.T) {
	s := mustSchedule(t, "09:00-18:00", "mon-fri")
	start := at(20, 17, 0) // 금 17:00 → 월 09:30까지
	f := newFake(start)
	simulate(newTestRunner(f, s), f, start, 64*time.Hour+30*time.Minute, nil)
	var friday, monday bool
	for _, n := range f.nudges {
		if !s.Active(n) {
			t.Fatalf("nudge at %s is outside the schedule", n.Format("Mon 15:04:05"))
		}
		friday = friday || n.Before(at(20, 18, 0))
		monday = monday || !n.Before(at(23, 9, 0))
	}
	if !friday || !monday {
		t.Fatalf("want nudges Fri before 18:00 and Mon after 09:00; got %d", len(f.nudges))
	}
}

func TestNudgeErrorIsRetried(t *testing.T) {
	t0 := at(25, 10, 0)
	f := newFake(t0.Add(-10 * time.Minute)) // 10분째 입력 없음
	*f.now = t0
	f.fail = 1
	r := newTestRunner(f, Schedule{})
	if d := r.Step(t0); d != PollCap || len(f.nudges) != 0 {
		t.Fatalf("after failure: sleep %s nudges %d; want PollCap and 0", d, len(f.nudges))
	}
	*f.now = t0.Add(PollCap)
	r.Step(*f.now)
	if len(f.nudges) != 1 {
		t.Fatalf("nudge not retried: %d nudges", len(f.nudges))
	}
}

func TestPickStaysInRange(t *testing.T) {
	r := newTestRunner(nil, Schedule{})
	lo, hi := 8*time.Minute, time.Duration(0)
	for range 10000 {
		d := r.pick()
		if d < time.Minute || d > 8*time.Minute {
			t.Fatalf("pick() = %s, outside [1m, 8m]", d)
		}
		lo, hi = min(lo, d), max(hi, d)
	}
	if lo > time.Minute+30*time.Second || hi < 8*time.Minute-30*time.Second {
		t.Fatalf("picks span only %s..%s", lo, hi)
	}
	r.Min, r.Max = 5*time.Minute, 5*time.Minute
	if d := r.pick(); d != 5*time.Minute {
		t.Fatalf("pick() with Min==Max = %s", d)
	}
}

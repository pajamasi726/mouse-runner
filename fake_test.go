package main

import (
	"errors"
	"time"
)

// 가짜 시계(*now)를 기준으로 유휴 시간을 계산하는 Input.
type fakeInput struct {
	now         *time.Time
	last        time.Time       // 마지막 입력 시각
	nudges      []time.Time     // 성공한 Nudge 시각
	idleAtNudge []time.Duration // 그때의 유휴 시간
	fail        int             // 앞으로 실패시킬 Nudge 횟수
	ignore      bool            // true면 Nudge가 유휴 시간을 되돌리지 않음
}

func newFake(start time.Time) *fakeInput {
	now := start
	return &fakeInput{now: &now, last: start}
}

func (f *fakeInput) Idle() time.Duration { return f.now.Sub(f.last) }

func (f *fakeInput) Nudge() error {
	if f.fail > 0 {
		f.fail--
		return errors.New("fake nudge failure")
	}
	f.idleAtNudge = append(f.idleAtNudge, f.Idle())
	f.nudges = append(f.nudges, *f.now)
	if !f.ignore {
		f.last = *f.now
	}
	return nil
}

// start부터 total 동안, r.Step이 돌려준 만큼 가짜 시계를 진행한다.
func simulate(r *Runner, f *fakeInput, start time.Time, total time.Duration, userActive func(time.Time) bool) (maxIdle, maxSleep time.Duration) {
	end := start.Add(total)
	for *f.now = start; f.now.Before(end); {
		if userActive != nil && userActive(*f.now) {
			f.last = *f.now
		}
		maxIdle = max(maxIdle, f.Idle())
		d := r.Step(*f.now)
		if d <= 0 {
			panic("Step returned a non-positive sleep")
		}
		maxSleep = max(maxSleep, d)
		*f.now = f.now.Add(d)
	}
	return maxIdle, maxSleep
}

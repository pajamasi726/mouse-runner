package main

import (
	"math/rand/v2"
	"time"
)

// Input은 OS 입력 장치를 다루는 최소 인터페이스다.
type Input interface {
	Idle() time.Duration // 마지막 입력 후 경과 시간
	Nudge() error        // 커서를 움직이지 않는 입력 신호 1회
}

// PollCap은 한 번에 자는 최대 시간이다. 잠자기에서 깨어나거나
// 근무 시간 경계를 지날 때 이 시간 안에 다시 확인한다.
const PollCap = 30 * time.Second

// nudgeTolerance: 입력 신호가 먹혔다면 다음 확인 때 유휴 시간은
// "그 신호 뒤로 지난 시간 + 이 값"을 넘지 않는다.
const nudgeTolerance = 5 * time.Second

type schedState int

const (
	stateUnknown schedState = iota
	stateOn
	stateOff
)

// Runner는 유휴 시간이 [Min, Max]에서 무작위로 뽑은 목표에 닿을 때마다
// 입력 신호를 한 번 보내고, 다음 목표를 새로 뽑는다.
type Runner struct {
	In    Input
	Sched Schedule
	Min   time.Duration
	Max   time.Duration
	Rand  *rand.Rand
	Logf  func(format string, args ...any)

	target    time.Duration
	state     schedState
	lastNudge time.Time // 아직 확인하지 않은 마지막 입력 신호 시각 (없으면 zero)
	ignored   bool      // 입력 신호가 먹히지 않는 중
}

// Step은 한 번 확인하고, 다음 확인까지 잘 시간을 돌려준다.
func (r *Runner) Step(now time.Time) time.Duration {
	if !r.Sched.Active(now) {
		if r.state != stateOff {
			r.Logf("근무 시간 밖 · 대기")
			r.state = stateOff
		}
		r.target = 0
		r.lastNudge = time.Time{}
		return PollCap
	}
	if r.state != stateOn {
		r.Logf("동작 중")
		r.state = stateOn
	}
	if r.target == 0 {
		r.target = r.pick()
	}
	idle := r.In.Idle()
	r.checkLastNudge(now, idle)
	if idle >= r.target {
		if err := r.In.Nudge(); err != nil {
			r.Logf("입력 신호 실패: %v · 30초 뒤 재시도", err)
			return PollCap
		}
		r.lastNudge = now
		r.target = r.pick()
		if !r.ignored {
			r.Logf("입력 신호 보냄 (유휴 %s) · 다음은 유휴 %s 때", idle.Round(time.Second), r.target.Round(time.Second))
		}
		return min(r.target, PollCap)
	}
	return min(r.target-idle, PollCap)
}

// checkLastNudge는 지난 입력 신호가 실제로 유휴 시간을 되돌렸는지 확인한다.
// 권한이 사라지거나 화면이 잠기면 이벤트가 조용히 무시되므로, 상태가 바뀔 때만 한 번씩 알린다.
// 경과 시간은 벽시계로 잰다(Round(0)). Go의 단조 시계는 잠자기 동안 멈춰서 깨어난 뒤 오경보를 낸다.
func (r *Runner) checkLastNudge(now time.Time, idle time.Duration) {
	if r.lastNudge.IsZero() {
		return
	}
	registered := idle <= now.Round(0).Sub(r.lastNudge.Round(0))+nudgeTolerance
	r.lastNudge = time.Time{}
	switch {
	case !registered && !r.ignored:
		r.ignored = true
		r.Logf("경고: 입력 신호가 먹히지 않습니다 (유휴 %s) · 손쉬운 사용 권한이나 화면 잠금 상태를 확인하세요", idle.Round(time.Second))
	case registered && r.ignored:
		r.ignored = false
		r.Logf("입력 신호가 다시 먹힙니다")
	}
}

// pick은 [Min, Max]에서 균등하게 목표 유휴 시간을 뽑는다.
func (r *Runner) pick() time.Duration {
	return r.Min + time.Duration(r.Rand.Int64N(int64(r.Max-r.Min)+1))
}

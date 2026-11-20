# mouse-runner 구현 계획

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Slack 데스크톱이 자리 비움으로 바뀌는 것을 막는 macOS용 Go CLI `mouse-runner`를 만들고, v0.1.0 유니버설 바이너리를 GitHub Release로 배포한다.

**Architecture:**
- 플랫폼과 무관한 코어는 `Input` 인터페이스와 가짜 시계로 단위 테스트한다. 코어에는 스케줄 판정, 무작위 목표 루프 `Runner.Step`, 설정 파싱, 셀프테스트가 들어간다.
- macOS 전용 부분은 cgo로 CoreGraphics를 호출하는 얇은 파일 하나에 둔다. 여기에는 유휴 시간 읽기, 제자리 마우스 이벤트, 권한 확인이 들어간다.
- `main`은 이 둘을 엮는다. `Step`이 돌려준 시간만큼 자고 다시 확인하는 일을 Ctrl+C까지 반복한다.

**Tech Stack:** Go 1.27 표준 라이브러리(`flag`, `math/rand/v2`, `os/signal`), cgo + CoreGraphics, Make, lipo, gh CLI

**Spec:** `docs/superpowers/specs/2026-11-20-mouse-runner-design.md`

## Global Constraints
- 모듈 경로는 `github.com/pajamasi726/mouse-runner`이다. 루트에 패키지 `main` 하나만 두고, 바이너리 이름은 `mouse-runner`이다. `go` 지시어는 `go mod init`이 쓰는 1.27.x 값을 그대로 쓴다.
- 외부 의존성은 없다(표준 라이브러리와 cgo만). 네트워크를 쓰지 않고 파일도 쓰지 않는다. 로그는 stdout/stderr로만 낸다.
- 기본값은 `-min 1m`, `-max 8m`, 대기 상한 `PollCap = 30 * time.Second`이다. 셀프테스트는 입력 없는 상태가 3초 될 때까지 최대 30초 기다린다.
- 이벤트는 현재 커서 위치에 `kCGEventMouseMoved` 하나를 `kCGHIDEventTap`으로 보낸다. 커서는 움직이지 않는다.
- 유휴 시간은 `CGEventSourceSecondsSinceLastEventType(kCGEventSourceStateCombinedSessionState, kCGAnyInputEventType)`으로 읽는다.
- 권한은 `CGPreflightPostEventAccess`로 확인한다. 없으면 `CGRequestPostEventAccess`로 요청하고, 안내를 출력한 뒤 종료 코드 1로 끝낸다.
- `-max`가 10분 이상이면 경고만 하고 오류로 처리하지 않는다. 사용자에게 보이는 문구는 한국어로 쓴다.
- macOS 배포 타깃은 12.0이고, arm64와 amd64를 묶은 유니버설 바이너리 하나로 낸다.

## Review Focus
1. **다른 터미널 앱에서 실행해 권한이 없을 때**(Terminal.app, iTerm2): 아무것도 하지 않은 채 조용히 돌면 안 된다. "손쉬운 사용" 안내를 출력하고 종료 코드 1로 끝낸다. → Task 4 `TestEnsureAccessExplainsMissingPermission`
2. **맥이 잠자기에 들어갔다 깨어날 때**: 깨어난 뒤 30초 안에 다시 확인한다. 유휴 시간이 목표를 넘었으면 곧바로 이벤트를 보낸다. → Task 2 `TestRecoversAfterSystemSleep`, `TestStepNeverSleepsLongerThanPollCap`
3. **사용자가 계속 컴퓨터를 쓰는 중일 때**: 이벤트를 한 번도 보내지 않는다. → Task 2 `TestNoNudgeWhileUserActive`
4. **기다리는 도중에 근무 시간이 끝날 때**: 근무 시간 밖에서는 이벤트를 보내지 않는다. → Task 2 `TestNoNudgeOutsideSchedule`
5. **이벤트 생성이 실패할 때**: 프로그램이 죽지 않는다. 로그를 남기고 30초 뒤 다시 시도한다. → Task 2 `TestNudgeErrorIsRetried`

---

### Task 1: 프로젝트 뼈대 + 근무 시간 스케줄

**Files:**
- Create: `go.mod`, `.gitignore`, `schedule.go`
- Test: `schedule_test.go`

**Interfaces:**
- Produces:
  - `type Schedule struct`. 제로 값은 "항상 동작"이다.
  - `func ParseSchedule(hours, days string) (Schedule, error)`
    - `hours`는 `"HH:MM-HH:MM"` 또는 `""`(종일)이다.
    - 끝 시각으로 `24:00`을 허용한다. 시작과 끝이 같으면 오류다. 시작이 끝보다 늦으면 자정을 넘는 구간으로 본다.
    - `days`는 쉼표로 구분한 `sun`~`sat` 이름이나 `a-b` 범위다. 대소문자는 무시하고, `fri-mon`처럼 주를 넘어가는 범위도 허용한다. `""`는 매일이다.
  - `func (s Schedule) Active(t time.Time) bool`: `t`의 요일과 시:분을 본다. 구간은 `[시작, 끝)`이다.
  - `func (s Schedule) String() string`: 둘 다 비었으면 `"항상"`, 아니면 `"<days|매일> <hours|종일>"`을 돌려준다.
  - 테스트 헬퍼 `at`, `mustSchedule`, `expectActive`(아래 코드). Task 2와 3에서도 쓴다.

- [ ] **Step 1: 뼈대 만들기.** `go mod init github.com/pajamasi726/mouse-runner`를 실행한다. `.gitignore`에는 `/dist/`, `/mouse-runner` 두 줄을 넣는다.
- [ ] **Step 2: 실패하는 테스트 작성** (`schedule_test.go`)

```go
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
```

- [ ] **Step 3: 실패 확인.** `go test ./...` → FAIL (`undefined: ParseSchedule`)
- [ ] **Step 4: `schedule.go` 구현.** 요일은 `time.Weekday`를 인덱스로 하는 `[7]bool`로, 시각은 자정부터 센 분으로 저장한다. 오류 문구는 한국어로 쓴다.
- [ ] **Step 5: 통과 확인.** `go vet ./... && go test ./...` → PASS
- [ ] **Step 6: 커밋.** `git add go.mod .gitignore schedule.go schedule_test.go && git commit -m "feat: work-hours schedule parsing"`

### Task 2: 무작위 목표 시간 루프 (Runner)

**Files:**
- Create: `runner.go`
- Test: `fake_test.go`, `runner_test.go`

**Interfaces:**
- Consumes: Task 1의 `Schedule`, `mustSchedule`, `at`
- Produces:
  - `type Input interface { Idle() time.Duration; Nudge() error }`
  - `const PollCap = 30 * time.Second`
  - `type Runner struct { In Input; Sched Schedule; Min, Max time.Duration; Rand *rand.Rand; Logf func(format string, args ...any); /* 비공개 상태: target, 마지막 스케줄 상태 */ }`. `Rand`는 `math/rand/v2`의 것이다.
  - `func (r *Runner) Step(now time.Time) time.Duration`: 한 번 확인하고, 다음 확인까지 잘 시간을 돌려준다.
  - `func (r *Runner) pick() time.Duration`은 `Min + Rand.Int64N(Max-Min+1)`이다.
  - 테스트 헬퍼 `fakeInput`, `newFake`, `simulate`(`fake_test.go`). Task 4에서도 쓴다.

`Step` 알고리즘 (스펙이 정한 부분):
```
if !Sched.Active(now): 상태가 바뀌었으면 Logf("근무 시간 밖 · 대기"); target = 0; return PollCap
상태가 바뀌었으면 Logf("동작 중")
if target == 0: target = pick()
idle := In.Idle()
if idle >= target:
    if err := In.Nudge(); err != nil { Logf(오류, "30초 뒤 재시도"); return PollCap }  // target 유지
    target = pick(); Logf(유휴 시간, 다음 목표); return min(target, PollCap)
return min(target - idle, PollCap)
```

- [ ] **Step 1: 테스트 헬퍼 작성** (`fake_test.go`)

```go
// 가짜 시계(*now)를 기준으로 유휴 시간을 계산하는 Input.
type fakeInput struct {
	now         *time.Time
	last        time.Time       // 마지막 입력 시각
	nudges      []time.Time     // 성공한 Nudge 시각
	idleAtNudge []time.Duration // 그때의 유휴 시간
	fail        int             // 앞으로 실패시킬 Nudge 횟수
	ignore      bool            // true면 Nudge가 유휴 시간을 되돌리지 않음
}

func newFake(start time.Time) *fakeInput { now := start; return &fakeInput{now: &now, last: start} }

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
```

- [ ] **Step 2: 실패하는 테스트 작성** (`runner_test.go`)

```go
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
```

- [ ] **Step 3: 실패 확인.** `go test ./...` → FAIL (`undefined: Runner`)
- [ ] **Step 4: `runner.go` 구현.** 위의 알고리즘을 따른다. 로그에 찍는 시간은 `Round(time.Second)`로 반올림한다.
- [ ] **Step 5: 통과 확인.** `go vet ./... && go test ./...` → PASS
- [ ] **Step 6: 커밋.** `git add runner.go runner_test.go fake_test.go && git commit -m "feat: random-interval runner loop"`

### Task 3: 설정(플래그) 파싱

**Files:**
- Create: `config.go`
- Test: `config_test.go`

**Interfaces:**
- Consumes: Task 1의 `ParseSchedule`, `Schedule`, `at`, `expectActive`
- Produces:
  - `type Config struct { Min, Max time.Duration; Sched Schedule; SelfTest, Version bool }`
  - `func ParseConfig(args []string, usage io.Writer) (Config, []string, error)`
    - 두 번째 반환값은 경고 문구 목록이다.
    - `flag.NewFlagSet("mouse-runner", flag.ContinueOnError)`을 쓰고, 출력은 `usage`로 보낸다.
    - `-h`면 `flag.ErrHelp`를 그대로 돌려준다.
  - 플래그와 기본값: `-min`(1m), `-max`(8m), `-hours`(""), `-days`(""), `-selftest`, `-version`
  - 오류 문구:
    - `-min은 0보다 커야 합니다`
    - `-min(%s)이 -max(%s)보다 큽니다`
  - 경고 문구: `-max가 10분 이상이면 그 사이에 Slack이 자리 비움으로 바뀔 수 있습니다`

- [ ] **Step 1: 실패하는 테스트 작성** (`config_test.go`)

```go
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
```

- [ ] **Step 2: 실패 확인.** `go test ./...` → FAIL (`undefined: ParseConfig`)
- [ ] **Step 3: `config.go` 구현**
- [ ] **Step 4: 통과 확인.** `go vet ./... && go test ./...` → PASS
- [ ] **Step 5: 커밋.** `git add config.go config_test.go && git commit -m "feat: command-line config"`

### Task 4: macOS 입력 + 셀프테스트 + main

**Files:**
- Create: `input_darwin.go`, `input_other.go`, `selftest.go`, `main.go`
- Test: `selftest_test.go`, `main_test.go`

**Interfaces:**
- Consumes:
  - Task 2: `Input`, `Runner`, `PollCap`, `fakeInput`, `newFake`
  - Task 3: `ParseConfig`, `Config`
- Produces:
  - `func newInput() (Input, error)`. darwin에서는 `macInput{}`를 돌려준다. 그 밖의 OS(`//go:build !darwin`)에서는 `mouse-runner는 현재 macOS만 지원합니다` 오류를 돌려준다.
  - `func postEventAccess() bool`, `func requestPostEventAccess() bool`. darwin이 아니면 둘 다 `false`다.
  - `func SelfTest(in Input, out io.Writer, sleep func(time.Duration)) bool`
    1. 250ms마다 확인하면서 유휴 시간이 3s 이상이 되기를 최대 30s 기다린다. 끝내 안 되면 `false`.
    2. `Nudge`를 보낸다.
    3. 200ms 뒤 유휴 시간을 다시 잰다. `after < time.Second && after < before`이면 성공이다.
  - `func ensureAccess(has, request func() bool, out io.Writer) bool`
    - `has()`가 참이면 아무것도 출력하지 않고 `true`.
    - 아니면 `request()`를 호출한다. 그 결과가 참이면 `true`.
    - 둘 다 아니면 안내를 출력하고 `false`. 안내에는 `시스템 설정 → 개인정보 보호 및 보안 → 손쉬운 사용`, 지금 쓰는 터미널 앱을 켠 뒤 다시 실행하라는 말을 넣는다.
  - `var version = "dev"` (`main.go`)
- cgo에서 고정할 값:
  - `#cgo LDFLAGS: -framework CoreGraphics -framework CoreFoundation`, `#include <CoreGraphics/CoreGraphics.h>`
  - 유휴 시간: `CGEventSourceSecondsSinceLastEventType(kCGEventSourceStateCombinedSessionState, kCGAnyInputEventType)`. 초 단위 double을 `time.Duration`으로 바꾼다.
  - Nudge 순서:
    1. `CGEventCreate(NULL)` → `CGEventGetLocation` → `CFRelease`
    2. `CGEventCreateMouseEvent(NULL, kCGEventMouseMoved, p, kCGMouseButtonLeft)` → `CGEventPost(kCGHIDEventTap, ev)` → `CFRelease`
    3. 이벤트가 NULL이면 0이 아닌 코드를 돌려주고, Go 쪽에서 error로 바꾼다.
- `main()` 흐름:
  1. `ParseConfig(os.Args[1:], os.Stderr)`: `ErrHelp`면 종료 코드 0, 다른 오류면 stderr에 출력하고 종료 코드 2. 경고는 stderr로 낸다.
  2. `-version`이면 `version`을 출력하고 0으로 끝낸다.
  3. `newInput` 오류면 1로 끝낸다.
  4. `ensureAccess(postEventAccess, requestPostEventAccess, os.Stderr)`가 실패하면 1로 끝낸다.
  5. `-selftest`면 `SelfTest(in, os.Stdout, time.Sleep)`의 결과에 따라 0 또는 1로 끝낸다.
  6. 시작 배너를 출력한다: `mouse-runner <version> · <min>~<max> 사이 무작위 · 일정: <Sched> · Ctrl+C로 종료`
  7. `signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)`로 신호를 받는다. `d := r.Step(time.Now())`를 실행하고, `ctx.Done()`과 `time.After(d)` 중 먼저 오는 쪽을 기다리는 일을 반복한다. 끝나면 `종료합니다`를 출력한다.
  8. `Logf`는 `15:04:05` 형식의 시각을 앞에 붙이고, `Rand`는 `rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))`로 만든다.

- [ ] **Step 1: 실패하는 테스트 작성** (`selftest_test.go`, `main_test.go`)

```go
// selftest_test.go
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

// main_test.go
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
```

- [ ] **Step 2: 실패 확인.** `go test ./...` → FAIL (`undefined: SelfTest`)
- [ ] **Step 3: `selftest.go`, `main.go`, `input_darwin.go`, `input_other.go` 구현**
- [ ] **Step 4: 통과 확인.**
  - `go vet ./... && go test ./...` → PASS
  - `GOOS=linux CGO_ENABLED=0 go build -o /dev/null .` → 성공 (darwin이 아닌 쪽 스텁이 컴파일되는지)
- [ ] **Step 5: 실제 동작 확인** (Ghostty 권한 기준)
  - `go build -o mouse-runner . && ./mouse-runner -version` → `dev`
  - `./mouse-runner -h` → 사용법이 나오고 종료 코드 0
  - `./mouse-runner -min 5m -max 2m` → 오류가 나고 종료 코드 2
  - `./mouse-runner -selftest` → 손을 떼고 기다리면 `성공`이 나오고 종료 코드 0
  - `./mouse-runner -min 3s -max 6s`를 40초쯤 백그라운드로 돌린다. 입력이 없었다면 로그에 유휴 3~6초 사이의 서로 다른 값으로 이벤트가 찍혀야 한다. 사용자가 계속 입력하고 있었다면 이벤트가 0건인 것이 정상이다.
- [ ] **Step 6: 커밋.** `git add input_darwin.go input_other.go selftest.go selftest_test.go main.go main_test.go && git commit -m "feat: macOS input, self-test, and CLI entry point"`

### Task 5: 빌드(유니버설 바이너리) + README

**Files:**
- Create: `Makefile`, `README.md`

**Interfaces:**
- Consumes: Task 4의 `main.version`
- Produces:
  - `make test`: `go vet` + `go test`
  - `make build`: 이 맥용 `./mouse-runner`
  - `make dist VERSION=vX.Y.Z`: `dist/mouse-runner_$(VERSION)_darwin_universal.tar.gz`(안에는 `mouse-runner` 하나만)와 `dist/checksums.txt`(`shasum -a 256`)
  - `make clean`
- Makefile에서 고정할 값:
  - `VERSION ?= dev`, `LDFLAGS = -s -w -X main.version=$(VERSION)`
  - `export MACOSX_DEPLOYMENT_TARGET = 12.0`, `CGO_ENABLED=1`, `-trimpath`
  - `GOARCH=arm64`와 `GOARCH=amd64`로 각각 빌드한 뒤 `lipo -create`로 하나로 합친다.
- README는 한국어로 쓰고 다음 내용을 담는다.
  - 소개: 무엇을 하는지, 네트워크와 계정 정보를 쓰지 않는다는 점
  - 설치:
    1. `gh release download v0.1.0 -R pajamasi726/mouse-runner -p '*.tar.gz'`
    2. `tar xzf`
    3. 브라우저로 받았다면 `xattr -d com.apple.quarantine mouse-runner`
  - 처음 확인: `-selftest`
  - 실행 예 3개: 기본, 근무 시간 지정, 범위 지정
  - 옵션 표
  - 권한: 터미널 앱 기준으로 적용되고, 다른 앱에서 돌리면 그 앱을 손쉬운 사용에 추가해야 한다
  - 동작 원리: 무작위 목표, 30초 상한
  - 주의:
    - 화면 보호기와 자동 잠금이 걸리지 않는다 → 자리를 뜰 때 ⌃⌘Q로 잠근다
    - 덮개를 닫거나 잠자기에 들어가면 자리 비움이 된다
    - 내 폰으로 확인하면 폰 때문에 활성이 되므로 동료에게 봐 달라고 한다
    - 잠금 화면 상태에서의 동작은 확인하지 않았다
  - 직접 빌드: `make test`, `make dist VERSION=v0.1.0`

- [ ] **Step 1: `Makefile` 작성**
- [ ] **Step 2: 빌드 확인**
  - `make test` → PASS
  - `make dist VERSION=v0.1.0` → 성공
- [ ] **Step 3: 산출물 확인**
  - `lipo -archs dist/mouse-runner` → `x86_64 arm64`
  - `vtool -show-build dist/mouse-runner`의 `minos`가 두 아키텍처 모두 12.0인지 본다. 빌드 머신의 OS 버전이 찍혀 있으면 실패다.
  - `codesign -v dist/mouse-runner` → 오류가 없어야 한다
  - `dist/mouse-runner -version`과 `arch -x86_64 dist/mouse-runner -version` → 둘 다 `v0.1.0`
  - `tar tzf dist/mouse-runner_v0.1.0_darwin_universal.tar.gz` → `mouse-runner`
  - `(cd dist && shasum -a 256 -c checksums.txt)` → `OK`
- [ ] **Step 4: `README.md` 작성** (위에 적은 내용)
- [ ] **Step 5: 커밋.** `git add Makefile README.md && git commit -m "build: universal binary packaging and README"`

### Task 6: 전체 리뷰 → 푸시 → v0.1.0 릴리스

**Files:** 없음. 저장소 밖에서만 작업한다(git 원격, GitHub Release).

- [ ] **Step 1: 리뷰.** 설계 문서 커밋 이후 전체 변경을 리뷰한다(superpowers:requesting-code-review). 지적된 점을 고치고, Task 5 Step 2~3을 다시 실행한다.
- [ ] **Step 2: 푸시**
  - `git push -u origin main`
  - `git tag -a v0.1.0 -m "v0.1.0" && git push origin v0.1.0`
- [ ] **Step 3: 릴리스 만들기.** `gh release create v0.1.0 dist/mouse-runner_v0.1.0_darwin_universal.tar.gz dist/checksums.txt -R pajamasi726/mouse-runner --title v0.1.0 --notes-file <scratchpad>/release-notes.md`
  - 릴리스 노트는 한국어로 쓴다. 담을 내용: 무엇이 들었는지, 받는 법, 테스트 3단계(`-selftest` → 실행 → 10분 넘게 손 떼고 동료에게 확인), 알려진 한계(macOS만 지원, 서명 안 됨, 잠금 화면 동작 미확인)
- [ ] **Step 4: 확인**
  - `gh release view v0.1.0 -R pajamasi726/mouse-runner --json assets`에 파일 2개가 있어야 한다.
  - 새 빈 임시 폴더에서 `gh release download v0.1.0 -R pajamasi726/mouse-runner`로 받는다.
  - `shasum -a 256 -c checksums.txt` → `OK`
  - 압축을 풀고 `./mouse-runner -version` → `v0.1.0`

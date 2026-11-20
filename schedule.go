package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Schedule는 mouse-runner가 동작할 요일과 시간대를 나타낸다.
// 제로 값은 "항상 동작"이다.
type Schedule struct {
	hasDays    bool
	days       [7]bool // time.Weekday가 인덱스
	daysText   string
	hasHours   bool
	start, end int // 자정부터 센 분. end는 24:00(1440)까지 허용
}

var dayNames = map[string]time.Weekday{
	"sun": time.Sunday, "mon": time.Monday, "tue": time.Tuesday, "wed": time.Wednesday,
	"thu": time.Thursday, "fri": time.Friday, "sat": time.Saturday,
}

// ParseSchedule은 -hours("HH:MM-HH:MM", ""=종일)와 -days("mon-fri", "mon,wed,fri", ""=매일)를 해석한다.
func ParseSchedule(hours, days string) (Schedule, error) {
	var s Schedule
	if hours = strings.TrimSpace(hours); hours != "" {
		start, end, err := parseHours(hours)
		if err != nil {
			return Schedule{}, err
		}
		s.hasHours, s.start, s.end = true, start, end
	}
	if days = strings.ToLower(strings.ReplaceAll(days, " ", "")); days != "" {
		mask, err := parseDays(days)
		if err != nil {
			return Schedule{}, err
		}
		s.hasDays, s.days, s.daysText = true, mask, days
	}
	return s, nil
}

func parseHours(v string) (start, end int, err error) {
	a, b, ok := strings.Cut(v, "-")
	if !ok {
		return 0, 0, fmt.Errorf("-hours %q: HH:MM-HH:MM 형식이어야 합니다 (예: 09:00-18:00)", v)
	}
	if start, err = parseClock(a, false); err != nil {
		return 0, 0, fmt.Errorf("-hours %q: %w", v, err)
	}
	if end, err = parseClock(b, true); err != nil {
		return 0, 0, fmt.Errorf("-hours %q: %w", v, err)
	}
	if start == end {
		return 0, 0, fmt.Errorf("-hours %q: 시작과 끝이 같습니다", v)
	}
	return start, end, nil
}

// parseClock은 "HH:MM"을 자정부터 센 분으로 바꾼다. allow24면 "24:00"을 허용한다.
func parseClock(v string, allow24 bool) (int, error) {
	hh, mm, ok := strings.Cut(strings.TrimSpace(v), ":")
	h, errH := strconv.Atoi(hh)
	m, errM := strconv.Atoi(mm)
	if !ok || errH != nil || errM != nil || len(mm) != 2 {
		return 0, fmt.Errorf("%q는 HH:MM 형식이 아닙니다", v)
	}
	if allow24 && h == 24 && m == 0 {
		return 24 * 60, nil
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, fmt.Errorf("%q는 올바른 시각이 아닙니다", v)
	}
	return h*60 + m, nil
}

func parseDays(v string) ([7]bool, error) {
	var mask [7]bool
	for _, part := range strings.Split(v, ",") {
		a, b, isRange := strings.Cut(part, "-")
		from, ok := dayNames[a]
		if !ok {
			return mask, fmt.Errorf("-days %q: %q는 요일이 아닙니다 (sun mon tue wed thu fri sat)", v, a)
		}
		if !isRange {
			mask[from] = true
			continue
		}
		to, ok := dayNames[b]
		if !ok {
			return mask, fmt.Errorf("-days %q: %q는 요일이 아닙니다 (sun mon tue wed thu fri sat)", v, b)
		}
		for d := from; ; d = (d + 1) % 7 {
			mask[d] = true
			if d == to {
				break
			}
		}
	}
	return mask, nil
}

// Active는 t(현지 시각)가 동작 구간 [시작, 끝)에 드는지 판단한다.
func (s Schedule) Active(t time.Time) bool {
	if s.hasDays && !s.days[t.Weekday()] {
		return false
	}
	if !s.hasHours {
		return true
	}
	m := t.Hour()*60 + t.Minute()
	if s.start < s.end {
		return m >= s.start && m < s.end
	}
	return m >= s.start || m < s.end // 자정을 넘는 구간
}

func (s Schedule) String() string {
	if !s.hasDays && !s.hasHours {
		return "항상"
	}
	days, hours := "매일", "종일"
	if s.hasDays {
		days = s.daysText
	}
	if s.hasHours {
		hours = fmt.Sprintf("%02d:%02d-%02d:%02d", s.start/60, s.start%60, s.end/60, s.end%60)
	}
	return days + " " + hours
}

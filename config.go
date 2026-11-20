package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"time"
)

// slackAway는 Slack 데스크톱이 자리 비움으로 바꾸는 무입력 시간이다.
const slackAway = 10 * time.Minute

// Config는 명령줄 옵션을 해석한 결과다.
type Config struct {
	Min, Max time.Duration
	Sched    Schedule
	SelfTest bool
	Version  bool
}

// ParseConfig는 명령줄 인자를 해석한다. 두 번째 반환값은 경고 문구 목록이다.
// -h를 받으면 사용법을 usage에 쓰고 flag.ErrHelp를 돌려준다.
func ParseConfig(args []string, usage io.Writer) (Config, []string, error) {
	var c Config
	var hours, days string
	fs := flag.NewFlagSet("mouse-runner", flag.ContinueOnError)
	fs.SetOutput(usage)
	fs.DurationVar(&c.Min, "min", time.Minute, "입력 신호를 보낼 최소 무입력 시간")
	fs.DurationVar(&c.Max, "max", 8*time.Minute, "입력 신호를 보낼 최대 무입력 시간 (Slack은 10분이면 자리 비움)")
	fs.StringVar(&hours, "hours", "", "동작할 시간대 HH:MM-HH:MM (예: 09:00-18:00, 비우면 종일)")
	fs.StringVar(&days, "days", "", "동작할 요일 (예: mon-fri 또는 mon,wed,fri, 비우면 매일)")
	fs.BoolVar(&c.SelfTest, "selftest", false, "권한과 입력 신호가 먹히는지 한 번 확인하고 끝냄")
	fs.BoolVar(&c.Version, "version", false, "버전을 출력하고 끝냄")
	fs.Usage = func() {
		fmt.Fprint(usage, "사용법: mouse-runner [옵션]\n\n"+
			"무입력 시간이 -min~-max 사이의 무작위 시점에 닿으면, 커서를 움직이지 않는\n"+
			"입력 신호를 한 번 보내 Slack이 자리 비움으로 바뀌지 않게 합니다.\n\n옵션:\n")
		fs.PrintDefaults()
		fmt.Fprint(usage, "\n예:\n  mouse-runner -selftest\n  mouse-runner\n  mouse-runner -hours 09:00-18:00 -days mon-fri\n")
	}
	if err := fs.Parse(args); err != nil {
		return Config{}, nil, err
	}
	if fs.NArg() > 0 {
		return Config{}, nil, fmt.Errorf("알 수 없는 인자 %q (옵션은 -days mon-fri처럼 이름을 붙여 씁니다)", fs.Args())
	}
	if c.Min <= 0 {
		return Config{}, nil, errors.New("-min은 0보다 커야 합니다")
	}
	if c.Min > c.Max {
		return Config{}, nil, fmt.Errorf("-min(%s)이 -max(%s)보다 큽니다", c.Min, c.Max)
	}
	sched, err := ParseSchedule(hours, days)
	if err != nil {
		return Config{}, nil, err
	}
	c.Sched = sched
	var warn []string
	if c.Max >= slackAway {
		warn = append(warn, "-max가 10분 이상이면 그 사이에 Slack이 자리 비움으로 바뀔 수 있습니다")
	}
	return c, warn, nil
}

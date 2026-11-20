// mouse-runner는 Slack 데스크톱이 자리 비움으로 바뀌지 않도록, 무입력 시간이
// 무작위 목표에 닿을 때마다 커서를 움직이지 않는 입력 신호를 보낸다.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var version = "dev" // 빌드할 때 -ldflags "-X main.version=..."로 바꾼다

func main() { os.Exit(run()) }

func run() int {
	cfg, warnings, err := ParseConfig(os.Args[1:], os.Stderr)
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "오류:", err)
		return 2
	}
	for _, w := range warnings {
		fmt.Fprintln(os.Stderr, "경고:", w)
	}
	if cfg.Version {
		fmt.Println(version)
		return 0
	}
	in, err := newInput()
	if err != nil {
		fmt.Fprintln(os.Stderr, "오류:", err)
		return 1
	}
	if !ensureAccess(postEventAccess, requestPostEventAccess, os.Stderr) {
		return 1
	}
	if cfg.SelfTest {
		if SelfTest(in, os.Stdout, time.Sleep) {
			return 0
		}
		return 1
	}

	fmt.Printf("mouse-runner %s · %s~%s 사이 무작위 · 일정: %s · Ctrl+C로 종료\n", version, cfg.Min, cfg.Max, cfg.Sched)
	r := &Runner{
		In: in, Sched: cfg.Sched, Min: cfg.Min, Max: cfg.Max,
		Rand: rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())),
		Logf: func(format string, args ...any) {
			fmt.Printf("%s %s\n", time.Now().Format("15:04:05"), fmt.Sprintf(format, args...))
		},
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	for {
		d := r.Step(time.Now())
		select {
		case <-ctx.Done():
			fmt.Println("종료합니다")
			return 0
		case <-time.After(d):
		}
	}
}

// ensureAccess는 입력 신호를 보낼 권한을 확인하고, 없으면 요청한 뒤 설정 방법을 안내한다.
func ensureAccess(has, request func() bool, out io.Writer) bool {
	if has() || request() {
		return true
	}
	fmt.Fprint(out, "입력 신호를 보낼 권한이 없습니다.\n"+
		"시스템 설정 → 개인정보 보호 및 보안 → 손쉬운 사용에서\n"+
		"지금 쓰는 터미널 앱(예: Ghostty, 터미널, iTerm)을 켠 뒤 다시 실행해 주세요.\n")
	return false
}

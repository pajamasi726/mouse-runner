package main

import (
	"fmt"
	"io"
	"time"
)

const (
	selfTestQuiet   = 3 * time.Second  // 이만큼 입력이 없어야 시험한다
	selfTestTimeout = 30 * time.Second // 이 안에 조용해지지 않으면 포기
	selfTestTick    = 250 * time.Millisecond
	selfTestSettle  = 200 * time.Millisecond
)

// SelfTest는 입력 신호 한 번이 실제로 유휴 시간을 0으로 되돌리는지 확인한다.
func SelfTest(in Input, out io.Writer, sleep func(time.Duration)) bool {
	fmt.Fprintln(out, "셀프테스트: 마우스와 키보드에서 손을 떼고 3초만 기다려 주세요...")
	var waited time.Duration
	for in.Idle() < selfTestQuiet {
		if waited >= selfTestTimeout {
			fmt.Fprintln(out, "실패: 30초 동안 입력이 계속 들어와서 시험하지 못했습니다. 손을 떼고 다시 실행해 주세요.")
			return false
		}
		sleep(selfTestTick)
		waited += selfTestTick
	}
	before := in.Idle()
	if err := in.Nudge(); err != nil {
		fmt.Fprintf(out, "실패: 입력 신호를 보내지 못했습니다: %v\n", err)
		return false
	}
	sleep(selfTestSettle)
	after := in.Idle()
	fmt.Fprintf(out, "유휴 시간 %s → %s\n", before.Round(100*time.Millisecond), after.Round(100*time.Millisecond))
	if after < time.Second && after < before {
		fmt.Fprintln(out, "성공: 입력 신호가 유휴 시간을 0으로 되돌렸습니다.")
		return true
	}
	fmt.Fprintln(out, "실패: 입력 신호가 유휴 시간을 되돌리지 못했습니다. 손쉬운 사용 권한을 확인해 주세요.")
	return false
}

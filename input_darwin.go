//go:build darwin

package main

/*
#cgo LDFLAGS: -framework CoreGraphics -framework CoreFoundation
#include <CoreGraphics/CoreGraphics.h>

static double mr_idle_seconds(void) {
	return CGEventSourceSecondsSinceLastEventType(
		kCGEventSourceStateCombinedSessionState, kCGAnyInputEventType);
}

// 현재 커서 위치에 마우스 이동 이벤트를 하나 보낸다. 커서는 움직이지 않는다.
static int mr_nudge(void) {
	CGEventRef here = CGEventCreate(NULL);
	if (here == NULL) return 1;
	CGPoint p = CGEventGetLocation(here);
	CFRelease(here);
	CGEventRef ev = CGEventCreateMouseEvent(NULL, kCGEventMouseMoved, p, kCGMouseButtonLeft);
	if (ev == NULL) return 2;
	CGEventPost(kCGHIDEventTap, ev);
	CFRelease(ev);
	return 0;
}

static int mr_has_access(void) { return CGPreflightPostEventAccess() ? 1 : 0; }
static int mr_request_access(void) { return CGRequestPostEventAccess() ? 1 : 0; }
*/
import "C"

import (
	"fmt"
	"time"
)

// macInput은 CoreGraphics로 유휴 시간을 읽고 입력 신호를 보낸다.
type macInput struct{}

func newInput() (Input, error) { return macInput{}, nil }

func (macInput) Idle() time.Duration {
	return time.Duration(float64(C.mr_idle_seconds()) * float64(time.Second))
}

func (macInput) Nudge() error {
	if rc := C.mr_nudge(); rc != 0 {
		return fmt.Errorf("마우스 이벤트를 만들지 못했습니다 (code %d)", int(rc))
	}
	return nil
}

func postEventAccess() bool        { return C.mr_has_access() == 1 }
func requestPostEventAccess() bool { return C.mr_request_access() == 1 }

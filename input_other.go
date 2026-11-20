//go:build !darwin

package main

import "errors"

func newInput() (Input, error) { return nil, errors.New("mouse-runner는 현재 macOS만 지원합니다") }

func postEventAccess() bool        { return false }
func requestPostEventAccess() bool { return false }

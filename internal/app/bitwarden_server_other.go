//go:build !linux

package app

import "os/exec"

func bindToParent(*exec.Cmd) {}

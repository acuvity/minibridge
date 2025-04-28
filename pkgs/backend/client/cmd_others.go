//go:build !darwin && !linux

package client

import (
	"os/exec"
	"syscall"
)

func setCaps(cmd *exec.Cmd, _ string, _ *creds) {
	cmd.SysProcAttr = &syscall.SysProcAttr{}
}

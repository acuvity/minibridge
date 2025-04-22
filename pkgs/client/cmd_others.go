//go:build !darwin && !linux

package client

import (
	"os/exec"
	"syscall"
)

func setCaps(cmd *exec.Cmd, chroot string, creds *creds) {
	cmd.SysProcAttr = &syscall.SysProcAttr{}
}

//go:build linux

package client

import (
	"os/exec"
	"syscall"
)

func setCaps(cmd *exec.Cmd, chroot string) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGKILL,
		Chroot:    chroot,
	}
}

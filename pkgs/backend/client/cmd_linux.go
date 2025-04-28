//go:build linux

package client

import (
	"os/exec"
	"syscall"
)

func setCaps(cmd *exec.Cmd, chroot string, creds *creds) {

	var screds *syscall.Credential
	if creds != nil {
		screds = &syscall.Credential{
			Uid:    creds.Uid,
			Gid:    creds.Gid,
			Groups: creds.Groups,
		}
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig:  syscall.SIGKILL,
		Chroot:     chroot,
		Credential: screds,
	}
}

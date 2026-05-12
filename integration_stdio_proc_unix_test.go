//go:build unix

package gomcp_test

import (
	"os/exec"
	"syscall"
)

// setSubprocessGroup places the command in its own process group so
// killSubprocessGroup can reach every descendant (including the
// grandchild binary spawned by `go run`). Called before cmd.Start().
func setSubprocessGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

// killSubprocessGroup sends SIGKILL to the negative PID, which unix
// resolves to the entire process group we created above. Falls back
// to a direct Kill if the group id lookup fails (e.g. the child has
// already exited on its own).
func killSubprocessGroup(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		_ = cmd.Process.Kill()
		return
	}
	_ = syscall.Kill(-pgid, syscall.SIGKILL)
}

//go:build !unix

package gomcp_test

import "os/exec"

// setSubprocessGroup is a no-op on platforms that do not expose unix
// process groups. Callers fall back to plain cmd.Process.Kill().
func setSubprocessGroup(_ *exec.Cmd) {}

// killSubprocessGroup signals the primary child only on non-unix
// platforms. `go run` grandchildren may leak, but that is the
// pre-existing behaviour and CI only runs on unix anyway.
func killSubprocessGroup(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
}

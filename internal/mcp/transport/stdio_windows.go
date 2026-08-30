//go:build windows

package transport

import "os/exec"

func setSysProcAttr(cmd *exec.Cmd) {
	// Windows process group handling if needed, otherwise no-op.
}

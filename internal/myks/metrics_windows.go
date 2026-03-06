//go:build windows

package myks

import (
	"os/exec"
)

func getCmdMaxRSS(cmd *exec.Cmd) int64 {
	return 0
}

//go:build !windows

package myks

import (
	"os/exec"
	"runtime"
	"syscall"
)

func getCmdMaxRSS(cmd *exec.Cmd) int64 {
	if cmd == nil || cmd.ProcessState == nil {
		return 0
	}
	sys := cmd.ProcessState.SysUsage()
	if sys == nil {
		return 0
	}
	if rusage, ok := sys.(*syscall.Rusage); ok {
		rss := int64(rusage.Maxrss)
		// On Linux, Maxrss is in kilobytes. On macOS, it is in bytes.
		if runtime.GOOS == "linux" {
			rss *= 1024
		}
		return rss
	}
	return 0
}

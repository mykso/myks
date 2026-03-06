//go:build windows

package myks

import (
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// processMemoryCounters matches Windows PROCESS_MEMORY_COUNTERS (psapi.h).
type processMemoryCounters struct {
	Cb                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
	PrivateUsage               uintptr
}

var (
	modpsapi                 = windows.NewLazySystemDLL("psapi.dll")
	procGetProcessMemoryInfo = modpsapi.NewProc("GetProcessMemoryInfo")
)

func getCmdMaxRSS(cmd *exec.Cmd) int64 {
	if cmd == nil || cmd.Process == nil {
		return 0
	}

	handle, err := windows.OpenProcess(
		windows.PROCESS_QUERY_INFORMATION|windows.PROCESS_VM_READ,
		false,
		uint32(cmd.Process.Pid),
	)
	if err != nil {
		return 0
	}
	defer windows.CloseHandle(handle)

	var counters processMemoryCounters
	counters.Cb = uint32(unsafe.Sizeof(counters))

	r1, _, err := syscall.SyscallN(
		procGetProcessMemoryInfo.Addr(),
		uintptr(handle),
		uintptr(unsafe.Pointer(&counters)),
		uintptr(counters.Cb),
	)
	if r1 == 0 {
		return 0
	}

	// PeakWorkingSetSize is the peak value in bytes of the resident set (working set).
	return int64(counters.PeakWorkingSetSize)
}

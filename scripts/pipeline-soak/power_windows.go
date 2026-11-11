package main

import (
	"fmt"
	"golang.org/x/sys/windows"
	"runtime"
)

// A thread-scoped, temporary sleep request; no persistent power setting is
// changed. It is cleared on normal return and by Windows on process exit.
func holdSystemAwake() (func(), error) {
	runtime.LockOSThread()
	set := windows.NewLazySystemDLL("kernel32.dll").NewProc("SetThreadExecutionState")
	previous, _, err := set.Call(0x80000001)
	if previous == 0 {
		runtime.UnlockOSThread()
		return nil, fmt.Errorf("temporary sleep request: %v", err)
	}
	return func() { _, _, _ = set.Call(previous); runtime.UnlockOSThread() }, nil
}

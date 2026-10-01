//go:build windows

package main

import "golang.org/x/sys/windows"

// lowerPriority keeps plips from stealing time from games.
func lowerPriority() {
	windows.SetPriorityClass(windows.CurrentProcess(), windows.BELOW_NORMAL_PRIORITY_CLASS)
}

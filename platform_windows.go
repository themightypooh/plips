//go:build windows

package main

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32               = windows.NewLazySystemDLL("user32.dll")
	procGetCursorPos     = user32.NewProc("GetCursorPos")
	procGetAsyncKeyState = user32.NewProc("GetAsyncKeyState")
	procSystemParamsInfo = user32.NewProc("SystemParametersInfoW")
)

// lowerPriority keeps plips from stealing time from games.
func lowerPriority() {
	windows.SetPriorityClass(windows.CurrentProcess(), windows.BELOW_NORMAL_PRIORITY_CLASS)
}

// globalCursor returns the cursor position in physical screen pixels, even
// when the window is click-through and not receiving mouse events.
func globalCursor() (int, int, bool) {
	var pt struct{ X, Y int32 }
	r, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	return int(pt.X), int(pt.Y), r != 0
}

// keyDown reads a key regardless of which window has focus.
func keyDown(vk int) bool {
	r, _, _ := procGetAsyncKeyState.Call(uintptr(vk))
	return r&0x8000 != 0
}

// workAreaBottom is the top of the taskbar in physical pixels (0 if unknown).
func workAreaBottom() int {
	var rc struct{ L, T, R, B int32 }
	const spiGetWorkArea = 0x0030
	r, _, _ := procSystemParamsInfo.Call(spiGetWorkArea, 0, uintptr(unsafe.Pointer(&rc)), 0)
	if r == 0 {
		return 0
	}
	return int(rc.B)
}

const (
	vkControl = 0x11
	vkAlt     = 0x12
)

//go:build !windows

package main

// Desktop-overlay helpers are Windows-only for now; these keep other builds
// compiling.

func lowerPriority()                 {}
func globalCursor() (int, int, bool) { return 0, 0, false }
func keyDown(vk int) bool            { return false }
func workAreaBottom() int            { return 0 }

const (
	vkControl = 0x11
	vkAlt     = 0x12
)

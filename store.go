package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"plips/sim"
)

// Settings is the user-editable config at %APPDATA%\Plips\settings.json.
type Settings struct {
	Mode         string `json:"mode"`         // "gallery", "room" or "desktop"
	DesktopScale int    `json:"desktopScale"` // screen pixels per plip pixel on the desktop
	RoomScale    int    `json:"roomScale"`
	ShowInfo     bool   `json:"showInfo"`
}

// SavedPlip is one pet as stored in pets.json.
type SavedPlip struct {
	Name    string                  `json:"name"`
	Genome  sim.Genome              `json:"genome"`
	Brain   [sim.NA][sim.NF]float32 `json:"brain"`
	Drives  [sim.NDrive]float32     `json:"drives"`
	Colours [][3]uint8              `json:"colours"`
	Age     int                     `json:"age"`
}

func dataDir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		base = "."
	}
	d := filepath.Join(base, "Plips")
	os.MkdirAll(d, 0o755)
	return d
}

func load(name string, v any) bool {
	b, err := os.ReadFile(filepath.Join(dataDir(), name))
	if err != nil {
		return false
	}
	return json.Unmarshal(b, v) == nil
}

func save(name string, v any) {
	b, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		return
	}
	p := filepath.Join(dataDir(), name)
	tmp := p + ".tmp"
	if os.WriteFile(tmp, b, 0o644) == nil {
		os.Rename(tmp, p)
	}
}

func loadSettings() Settings {
	s := Settings{Mode: "gallery", DesktopScale: 3, RoomScale: 4, ShowInfo: true}
	load("settings.json", &s)
	if s.DesktopScale < 1 {
		s.DesktopScale = 3
	}
	if s.RoomScale < 1 {
		s.RoomScale = 4
	}
	return s
}

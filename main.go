// Plips: little liquid-pixel creatures with learning brains.
package main

import (
	"errors"
	"fmt"
	"image/color"
	"log"
	"math"
	"math/rand"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"plips/sim"
)

const version = "v0.5"

type mode int

const (
	modeGallery mode = iota
	modeRoom
)

// Two bars along the top: tabs, then the current screen's buttons. Both
// stay on screen whatever the window size.
const topH, botH = 48, 50

type Game struct {
	set  Settings
	mode mode
	rng  *rand.Rand
	gal  *Gallery
	room *Room
	ui   UI

	sw, sh   int
	lastSave time.Time

	toast     string
	toastTime time.Time
	confirm   string // a destructive button waiting for its second click
	confirmAt time.Time
}

func main() {
	lowerPriority()
	g := &Game{set: loadSettings(), rng: rand.New(rand.NewSource(time.Now().UnixNano()))}
	g.gal = NewGallery(g.rng)
	var saved []SavedPlip
	load("pets.json", &saved)
	g.room = NewRoom(saved, g.rng)
	if g.set.Mode == "room" {
		g.mode = modeRoom
	}

	ebiten.SetWindowTitle("Plips " + version)
	ebiten.SetTPS(60)
	mw, mh := ebiten.Monitor().Size()
	ww, wh := min(1300, mw*9/10), min(860, mh*8/10)
	ebiten.SetWindowSize(ww, wh)
	ebiten.SetWindowPosition((mw-ww)/2, (mh-wh)/3)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	err := ebiten.RunGame(g)
	g.persist()
	if err != nil && !errors.Is(err, ebiten.Termination) {
		log.Fatal(err)
	}
}

func (g *Game) persist() {
	save("pets.json", g.room.Snapshot())
	g.set.Mode = map[mode]string{modeGallery: "gallery", modeRoom: "room"}[g.mode]
	save("settings.json", g.set)
}

func (g *Game) say(s string) { g.toast, g.toastTime = s, time.Now() }

// sure implements "click again to confirm" for destructive buttons.
func (g *Game) sure(key string) bool {
	if g.confirm == key && time.Since(g.confirmAt) < 3*time.Second {
		g.confirm = ""
		return true
	}
	g.confirm, g.confirmAt = key, time.Now()
	return false
}

func (g *Game) asking(key string) bool {
	return g.confirm == key && time.Since(g.confirmAt) < 3*time.Second
}

// view fits a w x h pixel canvas into the space between the bars.
func (g *Game) view(w, h int) (s, ox, oy float64) {
	aw, ah := float64(g.sw)-24, float64(g.sh-topH-botH)-24
	s = math.Min(aw/float64(w), ah/float64(h))
	if s >= 1 {
		s = math.Floor(s*2) / 2
	}
	return s, (float64(g.sw) - float64(w)*s) / 2, topH + botH + (float64(g.sh-topH-botH)-float64(h)*s)/2
}

func (g *Game) Update() error {
	u := &g.ui
	u.Begin()
	sw := float64(g.sw)
	u.Panel(rect{0, 0, sw, topH})
	u.Panel(rect{0, topH, sw, botH})

	// top bar: tabs
	x := 12.0
	// Advance x whether or not Gallery was clicked: if Room were declared at
	// the same spot, the same click would hit it too and flip straight back.
	c, w := u.Button(x, 9, "Gallery", g.mode == modeGallery, false)
	if c {
		g.mode = modeGallery
	}
	x += w + 6
	if c, _ := u.Button(x, 9, fmt.Sprintf("Room (%d)", len(g.room.Plips())), g.mode == modeRoom, false); c {
		g.mode = modeRoom
	}

	cx, cy := ebiten.CursorPosition()
	by := float64(topH) + 10
	switch g.mode {
	case modeGallery:
		g.galleryBar(by)
		s, ox, oy := g.view(galleryW, galleryH)
		sx, sy := float32((float64(cx)-ox)/s), float32((float64(cy)-oy)/s)
		inside := !u.OverUI() && sx >= 0 && sy >= 0 && sx < galleryW && sy < galleryH
		if g.gal.Update(sx, sy, inside, inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft),
			inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight)) {
			g.starToast()
		}
	case modeRoom:
		g.roomBar(by)
		s, ox, oy := g.view(sim.RoomW, sim.RoomH)
		sx, sy := float32((float64(cx)-ox)/s), float32((float64(cy)-oy)/s)
		inside := !u.OverUI() && sx >= 0 && sy >= 0 && sx < sim.RoomW && sy < sim.RoomH
		g.room.Update(sx, sy, inside,
			ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft),
			inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft),
			inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight))
	}
	if g.mode != modeRoom {
		g.room.w.Step() // the room keeps living while you browse
		g.room.w.Step()
	}
	if time.Since(g.lastSave) > time.Minute {
		save("pets.json", g.room.Snapshot())
		g.lastSave = time.Now()
	}
	return nil
}

func (g *Game) starToast() {
	if gn, ok := g.gal.Selected(); ok {
		if g.gal.IsStarred(gn) {
			g.say(fmt.Sprintf("Starred. You have %d under Starred", len(g.gal.stars)))
		} else {
			g.say("Unstarred")
		}
	}
}

func (g *Game) galleryBar(y float64) {
	u, gal := &g.ui, g.gal
	sel, has := gal.Selected()
	x := 12.0
	step := func(clicked bool, w float64) bool { x += w + 6; return clicked }

	if step(u.Button(x, y, "New batch", false, false)) {
		gal.NewBatch()
	}
	if step(u.Button(x, y, "Breed from this one", false, !has)) {
		gal.Breed()
	}
	starLabel := "Star"
	if has && gal.IsStarred(sel) {
		starLabel = "Unstar"
	}
	if step(u.Button(x, y, starLabel, false, !has)) {
		gal.ToggleStar()
		g.starToast()
	}
	if step(u.Button(x, y, "Put in room", false, !has || g.room.Full())) {
		if p := g.room.Hatch(sel); p != nil {
			g.say(p.Name + " moved into the room")
		}
	}

	// right side: which set is showing
	starred := fmt.Sprintf("Starred (%d)", len(gal.stars))
	rx := float64(g.sw) - 12 - ButtonWidth(starred)
	if c, _ := u.Button(rx, y, starred, gal.starView, false); c {
		gal.ShowStarred(true)
	}
	rx -= ButtonWidth("Batch") + 6
	if c, _ := u.Button(rx, y, "Batch", !gal.starView, false); c {
		gal.ShowStarred(false)
	}
}

func (g *Game) roomBar(y float64) {
	u, rm := &g.ui, g.room
	x := 12.0
	for c := 0; c < sim.NCol; c++ {
		col := sim.Colours[c].C
		if u.Swatch(x, y, color.RGBA{uint8(col[0]), uint8(col[1]), uint8(col[2]), 255}, rm.colour == c) {
			rm.colour = c
		}
		x += btnH + 4
	}
	x += 10
	step := func(clicked bool, w float64) bool { x += w + 6; return clicked }
	if step(u.Button(x, y, "Clear spills", false, false)) {
		rm.ClearSpills()
	}
	if step(u.Button(x, y, "Info", g.set.ShowInfo, false)) {
		g.set.ShowInfo = !g.set.ShowInfo
	}
	x += 14
	for _, p := range rm.Plips() {
		key := fmt.Sprintf("remove-%p", p)
		label := "×  " + p.Name
		if g.asking(key) {
			label = "Remove " + p.Name + "?"
		}
		if step(u.Button(x, y, label, false, false)) && g.sure(key) {
			rm.Remove(p)
			g.say(p.Name + " left the room")
			break
		}
	}
	if step(u.Button(x, y, "Add random", false, rm.Full())) {
		if p := rm.Hatch(sim.RandomGenome(g.rng)); p != nil {
			g.say(p.Name + " hatched")
		}
	}
	label := "Clear room"
	if g.asking("clear") {
		label = "Sure? Click again"
	}
	if step(u.Button(x, y, label, false, len(rm.Plips()) == 0)) && g.sure("clear") {
		rm.ClearAll()
		g.say("Room cleared")
	}
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{8, 9, 10, 255})
	cx, cy := ebiten.CursorPosition()
	switch g.mode {
	case modeGallery:
		ebiten.SetCursorMode(ebiten.CursorModeVisible)
		s, ox, oy := g.view(galleryW, galleryH)
		g.gal.Draw(screen, ox, oy, s)
	case modeRoom:
		s, ox, oy := g.view(sim.RoomW, sim.RoomH)
		sx, sy := float32((float64(cx)-ox)/s), float32((float64(cy)-oy)/s)
		inside := !g.ui.OverUI() && sx >= 0 && sy >= 0 && sx < sim.RoomW && sy < sim.RoomH
		if inside {
			ebiten.SetCursorMode(ebiten.CursorModeHidden)
		} else {
			ebiten.SetCursorMode(ebiten.CursorModeVisible)
		}
		g.room.Draw(screen, ox, oy, s, g.set.ShowInfo, inside, sx, sy)
	}
	g.ui.Draw(screen)

	// right side of the top bar: status or a recent message
	msg := ""
	switch g.mode {
	case modeGallery:
		msg = g.gal.Caption() + "   ·   click a plip to select it, right-click to star it"
	case modeRoom:
		msg = "tap: drop   hold: stream   right-click: splash   on a plip: left pets, right pokes"
	}
	if time.Since(g.toastTime) < 3*time.Second {
		msg = g.toast
	}
	drawText(screen, msg, float64(g.sw)-12-textWidth(msg, 13), 16, 13, colDim)
}

func (g *Game) Layout(w, h int) (int, int) {
	g.sw, g.sh = w, h
	return w, h
}

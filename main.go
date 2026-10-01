// Plips: little liquid-pixel creatures with learning brains.
package main

import (
	"errors"
	"image/color"
	"log"
	"math"
	"math/rand"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"plips/sim"
)

type mode int

const (
	modeGallery mode = iota
	modeRoom
	modeDesktop
)

var modeNames = map[mode]string{modeGallery: "gallery", modeRoom: "room", modeDesktop: "desktop"}

type Game struct {
	set   Settings
	mode  mode
	rng   *rand.Rand
	gal   *Gallery
	pets  *Pets
	saved []SavedPlip

	sw, sh     int // window size in device-independent pixels
	lastSave   time.Time
	passthru   bool
	prevGlobal map[int]bool
	quit       bool
}

func main() {
	lowerPriority()
	g := &Game{set: loadSettings(), rng: rand.New(rand.NewSource(time.Now().UnixNano())), prevGlobal: map[int]bool{}}
	g.gal = NewGallery(g.rng)
	if !load("pets.json", &g.saved) || len(g.saved) == 0 {
		g.saved = NewPair(g.gal.stars, g.rng)
	}
	ebiten.SetWindowTitle("Plips")
	ebiten.SetTPS(60)
	ebiten.SetRunnableOnUnfocused(true)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	start := map[string]mode{"gallery": modeGallery, "room": modeRoom, "desktop": modeDesktop}[g.set.Mode]
	g.switchTo(start)

	err := ebiten.RunGameWithOptions(g, &ebiten.RunGameOptions{ScreenTransparent: true})
	g.persist()
	if err != nil && !errors.Is(err, ebiten.Termination) {
		log.Fatal(err)
	}
}

// persist saves the pets (if they're running) and settings.
func (g *Game) persist() {
	if g.pets != nil {
		g.saved = g.pets.Snapshot()
	}
	save("pets.json", g.saved)
	g.set.Mode = modeNames[g.mode]
	save("settings.json", g.set)
}

func (g *Game) switchTo(m mode) {
	if g.pets != nil {
		g.saved = g.pets.Snapshot()
		save("pets.json", g.saved)
		g.pets = nil
	}
	g.mode = m
	mon := ebiten.Monitor()
	mw, mh := mon.Size()
	switch m {
	case modeGallery, modeRoom:
		ebiten.SetWindowMousePassthrough(false)
		g.passthru = false
		ebiten.SetWindowFloating(false)
		ebiten.SetWindowDecorated(true)
		w, h := galleryW*galleryScale, galleryH*galleryScale
		if m == modeRoom {
			w, h = sim.RoomW*g.set.RoomScale, sim.RoomH*g.set.RoomScale
			g.pets = NewPets(sim.RoomW, sim.RoomH, sim.RoomFloor, true, g.saved, g.rng)
		}
		if w > mw-40 || h > mh-80 { // keep it on screen on small monitors
			k := math.Min(float64(mw-40)/float64(w), float64(mh-80)/float64(h))
			w, h = int(float64(w)*k), int(float64(h)*k)
		}
		ebiten.SetWindowSize(w, h)
		ebiten.SetWindowPosition((mw-w)/2, (mh-h)/2)
	case modeDesktop:
		ebiten.SetWindowDecorated(false)
		ebiten.SetWindowFloating(true)
		ebiten.SetWindowPosition(0, 0)
		ebiten.SetWindowSize(mw, mh)
		s := g.set.DesktopScale
		floor := float32(mh/s) - 2
		if b := workAreaBottom(); b > 0 {
			floor = float32(float64(b)/mon.DeviceScaleFactor()/float64(s)) - 1
		}
		g.pets = NewPets(mw/s, mh/s, floor, false, g.saved, g.rng)
	}
	ebiten.SetCursorMode(ebiten.CursorModeVisible)
	g.set.Mode = modeNames[m]
	save("settings.json", g.set)
}

// globalPressed edge-detects a Ctrl+Alt+key hotkey that works in any app.
func (g *Game) globalPressed(vk int) bool {
	down := keyDown(vkControl) && keyDown(vkAlt) && keyDown(vk)
	was := g.prevGlobal[vk]
	g.prevGlobal[vk] = down
	return down && !was
}

// view returns the scale and offset that fit a w x h pixel canvas in the window.
func (g *Game) view(w, h int) (s, ox, oy float64) {
	if g.mode == modeDesktop {
		s = float64(g.set.DesktopScale)
		return s, 0, 0
	}
	s = math.Min(float64(g.sw)/float64(w), float64(g.sh)/float64(h))
	if s >= 1 {
		s = math.Floor(s*4) / 4
	}
	return s, (float64(g.sw) - float64(w)*s) / 2, (float64(g.sh) - float64(h)*s) / 2
}

func (g *Game) cursor() (float64, float64) {
	if g.mode == modeDesktop {
		if x, y, ok := globalCursor(); ok {
			d := ebiten.Monitor().DeviceScaleFactor()
			wx, wy := ebiten.WindowPosition()
			return float64(x)/d - float64(wx), float64(y)/d - float64(wy)
		}
	}
	x, y := ebiten.CursorPosition()
	return float64(x), float64(y)
}

func (g *Game) Update() error {
	// mode hotkeys: F1-F3 when focused, Ctrl+Alt+1-3 from anywhere
	for k, m := range map[ebiten.Key]mode{ebiten.KeyF1: modeGallery, ebiten.KeyF2: modeRoom, ebiten.KeyF3: modeDesktop} {
		if inpututil.IsKeyJustPressed(k) && g.mode != m {
			g.switchTo(m)
		}
	}
	for vk, m := range map[int]mode{'1': modeGallery, '2': modeRoom, '3': modeDesktop} {
		if g.globalPressed(vk) && g.mode != m {
			g.switchTo(m)
		}
	}
	if g.globalPressed('Q') {
		return ebiten.Termination
	}
	if g.globalPressed('H') || inpututil.IsKeyJustPressed(ebiten.KeyTab) {
		g.set.ShowInfo = !g.set.ShowInfo
	}

	cx, cy := g.cursor()
	switch g.mode {
	case modeGallery:
		s, ox, oy := g.view(galleryW, galleryH)
		g.gal.Update(float32((cx-ox)/s), float32((cy-oy)/s))
	default:
		p := g.pets
		s, ox, oy := g.view(p.W, p.H)
		sx, sy := float32((cx-ox)/s), float32((cy-oy)/s)
		inside := sx >= 0 && sy >= 0 && sx < float32(p.W) && sy < float32(p.H)
		if g.mode == modeDesktop {
			// click-through unless over a plip/liquid or Ctrl+Alt is held
			grab := keyDown(vkControl) && keyDown(vkAlt)
			want := !(grab || p.Hit(sx, sy))
			if want != g.passthru {
				ebiten.SetWindowMousePassthrough(want)
				g.passthru = want
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyN) {
			g.saved = NewPair(g.gal.stars, g.rng)
			g.pets = nil
			g.switchTo(g.mode)
			return nil
		}
		_, wy := ebiten.Wheel()
		p.Update(sx, sy, inside,
			ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft),
			ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight),
			inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight),
			inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft),
			wy)
		if time.Since(g.lastSave) > time.Minute {
			g.saved = p.Snapshot()
			save("pets.json", g.saved)
			g.lastSave = time.Now()
		}
	}
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	cx, cy := g.cursor()
	switch g.mode {
	case modeGallery:
		screen.Fill(color.RGBA{8, 9, 10, 255})
		s, ox, oy := g.view(galleryW, galleryH)
		g.gal.Draw(screen, ox, oy, s)
	case modeRoom:
		screen.Fill(color.RGBA{8, 9, 10, 255})
		p := g.pets
		s, ox, oy := g.view(p.W, p.H)
		sx, sy := float32((cx-ox)/s), float32((cy-oy)/s)
		inside := sx >= 0 && sy >= 0 && sx < float32(p.W) && sy < float32(p.H)
		ebiten.SetCursorMode(map[bool]ebiten.CursorModeType{true: ebiten.CursorModeHidden, false: ebiten.CursorModeVisible}[inside])
		p.Draw(screen, ox, oy, s, g.set.ShowInfo, inside, sx, sy)
	case modeDesktop:
		p := g.pets
		s, ox, oy := g.view(p.W, p.H)
		sx, sy := float32((cx-ox)/s), float32((cy-oy)/s)
		grab := keyDown(vkControl) && keyDown(vkAlt)
		p.Draw(screen, ox, oy, s, g.set.ShowInfo, grab, sx, sy)
	}
}

func (g *Game) Layout(w, h int) (int, int) {
	g.sw, g.sh = w, h
	return w, h
}

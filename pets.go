package main

import (
	"fmt"
	"image/color"
	"math/rand"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"plips/sim"
)

// Pets is the live pair of plips with brains, in the room or on the desktop.
type Pets struct {
	w      *sim.World
	r      *sim.Renderer
	bg     []byte // nil on the desktop (transparent)
	buf    []byte
	img    *ebiten.Image
	W, H   int
	colour int

	// mouse state
	leftHeld   int
	leftOnPlip bool
}

func NewPets(w, h int, floor float32, room bool, saved []SavedPlip, rng *rand.Rand) *Pets {
	p := &Pets{W: w, H: h, buf: make([]byte, w*h*4), img: ebiten.NewImage(w, h), colour: 4}
	p.w = sim.NewWorld(float32(w), float32(h), floor, rng.Int63())
	p.r = sim.NewRenderer(w, h)
	if room {
		p.bg = sim.RoomBackground()
		p.w.Platforms = sim.RoomPlatforms
	}
	for i, s := range saved {
		pl := sim.NewPlip(s.Genome, s.Name, true)
		pl.Brain.W = s.Brain
		pl.Drives = s.Drives
		pl.Age = s.Age
		x := float32(w) * (0.35 + 0.3*float32(i))
		p.w.AddPlip(pl, x, s.Colours)
	}
	return p
}

// NewPair hatches two fresh plips, from starred favourites when there are any.
func NewPair(stars []sim.Genome, rng *rand.Rand) []SavedPlip {
	var out []SavedPlip
	for i := 0; i < 2; i++ {
		var g sim.Genome
		switch {
		case len(stars) >= 2:
			g = stars[len(stars)-2+i].Mutate(rng, 0.15)
		case len(stars) == 1:
			g = stars[0].Mutate(rng, 0.3)
		default:
			g = sim.RandomGenome(rng)
		}
		b := sim.NewBrain(g)
		out = append(out, SavedPlip{
			Name: sim.RandomName(rng), Genome: g, Brain: b.W,
			Drives: [sim.NDrive]float32{0.3, 0, 0.3, 0.2, 0.1},
		})
	}
	return out
}

// Snapshot captures the pets for saving or for moving between modes.
func (p *Pets) Snapshot() []SavedPlip {
	var out []SavedPlip
	for _, pl := range p.w.Plips {
		out = append(out, SavedPlip{
			Name: pl.Name, Genome: pl.G, Brain: pl.Brain.W, Drives: pl.Drives,
			Colours: p.w.BodyColours(pl), Age: pl.Age,
		})
	}
	return out
}

// Update steps the world and handles the mouse. (sx, sy) is the cursor in
// world pixels; inside reports whether the cursor is over the play area.
func (p *Pets) Update(sx, sy float32, inside, left, right, rightJust, leftJust bool, wheel float64) {
	w := p.w
	w.Hand.X, w.Hand.Y, w.Hand.Visible = sx, sy, inside

	for k := 0; k < sim.NCol; k++ {
		if ebiten.IsKeyPressed(ebiten.KeyDigit1 + ebiten.Key(k)) {
			p.colour = k
		}
	}
	if wheel > 0 {
		p.colour = (p.colour + sim.NCol - 1) % sim.NCol
	} else if wheel < 0 {
		p.colour = (p.colour + 1) % sim.NCol
	}

	if inside {
		if leftJust {
			p.leftHeld = 0
			if pl := w.PlipAt(sx, sy); pl != nil {
				pl.Pet()
				p.leftOnPlip = true
			} else {
				p.leftOnPlip = false
				if !w.Full() {
					w.Drop(sx, sy, p.colour)
				}
			}
		}
		if left && !p.leftOnPlip {
			p.leftHeld++
			if p.leftHeld > 9 && !w.Full() { // held: a steady stream
				w.Stream(sx, sy, p.colour)
				w.Stream(sx, sy+0.7, p.colour)
			}
		}
		if rightJust {
			if pl := w.PlipAt(sx, sy); pl != nil {
				pl.Poke(w, sx, sy)
			} else if !w.Full() {
				w.Splash(sx, sy, p.colour, 30)
			}
		}
	}
	if !left {
		p.leftHeld = 0
	}
	w.Step()
	w.Step()
}

// Hit reports whether a drawn pixel (plip or liquid) is at (x, y).
func (p *Pets) Hit(x, y float32) bool { return p.r.Hit(int(x), int(y)) }

func (p *Pets) Draw(screen *ebiten.Image, ox, oy, s float64, info bool, cursor bool, sx, sy float32) {
	p.r.Render(p.w, p.buf, p.bg)
	if cursor {
		c := sim.Colours[p.colour].C
		x, y := int(sx), int(sy)
		for _, d := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
			setPx(p.buf, p.W, p.H, x+d[0], y+d[1], [3]uint8{20, 20, 22})
		}
		setPx(p.buf, p.W, p.H, x, y, [3]uint8{uint8(c[0]), uint8(c[1]), uint8(c[2])})
	}
	p.img.WritePixels(p.buf)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(s, s)
	op.GeoM.Translate(ox, oy)
	screen.DrawImage(p.img, op)
	if info {
		p.drawInfo(screen, ox, oy)
	}
}

func setPx(buf []byte, w, h, x, y int, c [3]uint8) {
	if x < 0 || y < 0 || x >= w || y >= h {
		return
	}
	o := (y*w + x) * 4
	buf[o], buf[o+1], buf[o+2], buf[o+3] = c[0], c[1], c[2], 255
}

func (p *Pets) drawInfo(screen *ebiten.Image, ox, oy float64) {
	x := int(ox) + 10
	for _, pl := range p.w.Plips {
		y := int(oy) + 8
		vector.FillRect(screen, float32(x-4), float32(y-2), 236, 112, color.RGBA{10, 12, 13, 170}, false)
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%s  %d drops", pl.Name, pl.Mass), x, y)
		ebitenutil.DebugPrintAt(screen, pl.Doing(p.w), x, y+16)
		for d := 0; d < sim.NDrive; d++ {
			yy := y + 36 + d*10
			ebitenutil.DebugPrintAt(screen, sim.DriveNames[d], x, yy-4)
			vector.FillRect(screen, float32(x+52), float32(yy+1), 100, 5, color.RGBA{40, 44, 46, 255}, false)
			vector.FillRect(screen, float32(x+52), float32(yy+1), 100*pl.Drives[d], 5, color.RGBA{160, 180, 168, 255}, false)
		}
		ebitenutil.DebugPrintAt(screen, likes(pl), x, y+88)
		x += 250
	}
	c := sim.Colours[p.colour]
	hy := int(oy) + 124
	vector.FillRect(screen, float32(int(ox)+6), float32(hy), 12, 12, color.RGBA{uint8(c.C[0]), uint8(c.C[1]), uint8(c.C[2]), 255}, false)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%s  [1-8/wheel]  tap: drop  hold: stream  right: splash  on a plip: left pet, right poke  Tab: info  N: new pair", c.Name), int(ox)+24, hy-2)
}

func likes(pl *sim.Plip) string {
	type kv struct {
		c int
		v float32
	}
	var l []kv
	for c := 0; c < sim.NCol; c++ {
		l = append(l, kv{c, pl.Brain.Liking(c)})
	}
	sort.Slice(l, func(i, j int) bool { return l[i].v > l[j].v })
	var top []string
	for _, e := range l[:2] {
		top = append(top, strings.ToLower(sim.Colours[e.c].Name))
	}
	return "likes " + strings.Join(top, ", ") + "  meh on " + strings.ToLower(sim.Colours[l[len(l)-1].c].Name)
}

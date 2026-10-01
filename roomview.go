package main

import (
	"fmt"
	"image/color"
	"math/rand"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"plips/sim"
)

const maxPlips = 4

// Room is the live room where plips with brains get fed and taught.
type Room struct {
	w      *sim.World
	r      *sim.Renderer
	bg     []byte
	buf    []byte
	img    *ebiten.Image
	rng    *rand.Rand
	colour int

	leftHeld   int
	leftOnPlip bool
}

func NewRoom(saved []SavedPlip, rng *rand.Rand) *Room {
	rm := &Room{
		w:      sim.NewWorld(sim.RoomW, sim.RoomH, sim.RoomFloor, rng.Int63()),
		r:      sim.NewRenderer(sim.RoomW, sim.RoomH),
		bg:     sim.RoomBackground(),
		buf:    make([]byte, sim.RoomW*sim.RoomH*4),
		img:    ebiten.NewImage(sim.RoomW, sim.RoomH),
		rng:    rng,
		colour: 4,
	}
	rm.w.Platforms = sim.RoomPlatforms
	for _, s := range saved {
		rm.add(s)
	}
	return rm
}

func (rm *Room) Full() bool         { return len(rm.w.Plips) >= maxPlips }
func (rm *Room) Plips() []*sim.Plip { return rm.w.Plips }

func (rm *Room) add(s SavedPlip) *sim.Plip {
	pl := sim.NewPlip(s.Genome, s.Name, true)
	pl.Brain.W = s.Brain
	pl.Drives = s.Drives
	pl.Age = s.Age
	switch {
	case s.Limbs != nil:
		pl.SetLimbGrowth(s.Limbs)
	case s.Age == 0:
		pl.Bud() // hatchlings start with buds that grow in
	}
	if s.Motor != nil && pl.Motor != nil {
		pl.Motor = s.Motor
	}
	x := 30 + rm.rng.Float32()*(sim.RoomW-60)
	rm.w.AddPlip(pl, x, s.Colours)
	return pl
}

// Hatch adds a new plip with a fresh brain from a genome.
func (rm *Room) Hatch(g sim.Genome) *sim.Plip {
	if rm.Full() {
		return nil
	}
	return rm.add(SavedPlip{
		Name: sim.RandomName(rm.rng), Genome: g, Brain: sim.NewBrain(g).W,
		Drives: [sim.NDrive]float32{0.3, 0, 0.3, 0.2, 0.1},
	})
}

func (rm *Room) Remove(p *sim.Plip) { rm.w.RemovePlip(p) }

func (rm *Room) ClearAll() {
	for len(rm.w.Plips) > 0 {
		rm.w.RemovePlip(rm.w.Plips[0])
	}
}

func (rm *Room) ClearSpills() { rm.w.ClearLoose() }

// Snapshot captures the plips for saving.
func (rm *Room) Snapshot() []SavedPlip {
	out := []SavedPlip{}
	for _, pl := range rm.w.Plips {
		out = append(out, SavedPlip{
			Name: pl.Name, Genome: pl.G, Brain: pl.Brain.W, Drives: pl.Drives,
			Colours: rm.w.BodyColours(pl), Age: pl.Age,
			Limbs: pl.LimbGrowth(), Motor: pl.Motor,
		})
	}
	return out
}

// Update steps the world and handles the mouse over the room. (sx, sy) is
// the cursor in room pixels.
func (rm *Room) Update(sx, sy float32, inside, left, leftJust, rightJust bool) {
	w := rm.w
	w.Hand.X, w.Hand.Y, w.Hand.Visible = sx, sy, inside
	if inside {
		if leftJust {
			rm.leftHeld = 0
			if pl := w.PlipAt(sx, sy); pl != nil {
				pl.Pet()
				rm.leftOnPlip = true
			} else {
				rm.leftOnPlip = false
				if !w.Full() {
					w.Drop(sx, sy, rm.colour)
				}
			}
		}
		if left && !rm.leftOnPlip {
			rm.leftHeld++
			if rm.leftHeld > 9 && !w.Full() { // held: a steady stream
				w.Stream(sx, sy, rm.colour)
				w.Stream(sx, sy+0.7, rm.colour)
			}
		}
		if rightJust {
			if pl := w.PlipAt(sx, sy); pl != nil {
				pl.Poke(w, sx, sy)
			} else if !w.Full() {
				w.Splash(sx, sy, rm.colour, 30)
			}
		}
	}
	if !left {
		rm.leftHeld = 0
	}
	w.Step()
	w.Step()
}

func (rm *Room) Draw(screen *ebiten.Image, ox, oy, s float64, info, cursor bool, sx, sy float32) {
	rm.r.Render(rm.w, rm.buf, rm.bg)
	if cursor {
		c := sim.Colours[rm.colour].C
		x, y := int(sx), int(sy)
		for _, d := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
			setPx(rm.buf, x+d[0], y+d[1], [3]uint8{20, 20, 22})
		}
		setPx(rm.buf, x, y, [3]uint8{uint8(c[0]), uint8(c[1]), uint8(c[2])})
	}
	rm.img.WritePixels(rm.buf)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(s, s)
	op.GeoM.Translate(ox, oy)
	screen.DrawImage(rm.img, op)

	if len(rm.w.Plips) == 0 {
		drawText(screen, "The room is empty. Put some in from the Gallery, or press Add random.", ox+16, oy+14, 15, colText)
		return
	}
	if info {
		x := ox + 10
		for _, pl := range rm.w.Plips {
			rm.drawCard(screen, pl, x, oy+10)
			x += 222
		}
	}
}

func (rm *Room) drawCard(screen *ebiten.Image, pl *sim.Plip, x, y float64) {
	vector.FillRect(screen, float32(x), float32(y), 212, 146, color.RGBA{12, 14, 15, 200}, false)
	drawText(screen, fmt.Sprintf("%s   %d drops", pl.Name, pl.Mass), x+10, y+6, 14, colText)
	drawText(screen, pl.Doing(rm.w), x+10, y+24, 13, colAccent)
	for d := 0; d < sim.NDrive; d++ {
		yy := y + 46 + float64(d)*13
		drawText(screen, sim.DriveNames[d], x+10, yy-3, 11, colDim)
		vector.FillRect(screen, float32(x+60), float32(yy+2), 140, 5, color.RGBA{40, 44, 46, 255}, false)
		vector.FillRect(screen, float32(x+60), float32(yy+2), 140*pl.Drives[d], 5, color.RGBA{160, 180, 168, 255}, false)
	}
	drawText(screen, likes(pl), x+10, y+112, 11, colDim)
	if n, grown := pl.Legs(); n > 0 {
		drawText(screen, fmt.Sprintf("%d legs, %d%% grown, %s", n, int(grown*100), pl.WalkWords()), x+10, y+127, 11, colDim)
	}
}

func setPx(buf []byte, x, y int, c [3]uint8) {
	if x < 0 || y < 0 || x >= sim.RoomW || y >= sim.RoomH {
		return
	}
	o := (y*sim.RoomW + x) * 4
	buf[o], buf[o+1], buf[o+2], buf[o+3] = c[0], c[1], c[2], 255
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
	return fmt.Sprintf("likes %s, %s   meh on %s",
		strings.ToLower(sim.Colours[l[0].c].Name), strings.ToLower(sim.Colours[l[1].c].Name),
		strings.ToLower(sim.Colours[l[len(l)-1].c].Name))
}

package main

import (
	"fmt"
	"image/color"
	"math/rand"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"plips/sim"
)

// The gallery shows a grid of baby plips with no brains, for finding a look.
const (
	cellW, cellH, cellFloor = 108, 80, 70
	gCols, gRows            = 4, 3
	galleryW, galleryH      = cellW * gCols, cellH * gRows
	galleryScale            = 3
)

type cell struct {
	g       sim.Genome
	w       *sim.World
	r       *sim.Renderer
	starred bool
}

type Gallery struct {
	cells  [gCols * gRows]*cell
	bg     []byte
	buf    []byte
	canvas []byte
	img    *ebiten.Image
	rng    *rand.Rand
	stars  []sim.Genome
	hover  int
	gen    int
}

func NewGallery(rng *rand.Rand) *Gallery {
	g := &Gallery{
		bg:     sim.CellBackground(cellW, cellH, cellFloor),
		buf:    make([]byte, cellW*cellH*4),
		canvas: make([]byte, galleryW*galleryH*4),
		img:    ebiten.NewImage(galleryW, galleryH),
		rng:    rng,
		hover:  -1,
	}
	load("favourites.json", &g.stars)
	for i := range g.cells {
		g.set(i, sim.RandomGenome(rng))
	}
	return g
}

func (g *Gallery) set(i int, gn sim.Genome) {
	w := sim.NewWorld(cellW, cellH, cellFloor, g.rng.Int63())
	w.AddPlip(sim.NewPlip(gn, "", false), cellW/2, nil)
	for k := 0; k < 120; k++ {
		w.Step()
	}
	g.cells[i] = &cell{g: gn, w: w, r: sim.NewRenderer(cellW, cellH), starred: g.isStarred(gn)}
}

func (g *Gallery) isStarred(gn sim.Genome) bool {
	for _, s := range g.stars {
		if s.Instinct == gn.Instinct {
			return true
		}
	}
	return false
}

func (g *Gallery) toggleStar(i int) {
	c := g.cells[i]
	if c.starred {
		for k, s := range g.stars {
			if s.Instinct == c.g.Instinct {
				g.stars = append(g.stars[:k], g.stars[k+1:]...)
				break
			}
		}
	} else {
		g.stars = append(g.stars, c.g)
	}
	c.starred = !c.starred
	save("favourites.json", g.stars)
}

// Update handles input; sx, sy are cursor coords in gallery pixels.
func (g *Gallery) Update(sx, sy float32) {
	g.hover = -1
	if sx >= 0 && sy >= 0 && sx < galleryW && sy < galleryH {
		g.hover = int(sy)/cellH*gCols + int(sx)/cellW
	}
	if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		for i := range g.cells {
			g.set(i, sim.RandomGenome(g.rng))
		}
		g.gen = 0
	}
	if g.hover >= 0 {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			parent := g.cells[g.hover]
			for i := range g.cells {
				if i != g.hover {
					g.set(i, parent.g.Mutate(g.rng, 0.55))
				}
			}
			g.gen++
		}
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
			g.toggleStar(g.hover)
		}
	}
	for _, c := range g.cells {
		c.w.Step()
		c.w.Step()
	}
}

func (g *Gallery) Draw(screen *ebiten.Image, ox, oy, s float64) {
	for i, c := range g.cells {
		c.r.Render(c.w, g.buf, g.bg)
		x0, y0 := (i%gCols)*cellW, (i/gCols)*cellH
		for y := 0; y < cellH; y++ {
			copy(g.canvas[((y0+y)*galleryW+x0)*4:], g.buf[y*cellW*4:(y+1)*cellW*4])
		}
	}
	g.img.WritePixels(g.canvas)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(s, s)
	op.GeoM.Translate(ox, oy)
	screen.DrawImage(g.img, op)

	for i, c := range g.cells {
		x := float32(ox) + float32((i%gCols)*cellW)*float32(s)
		y := float32(oy) + float32((i/gCols)*cellH)*float32(s)
		cw, ch := float32(cellW)*float32(s), float32(cellH)*float32(s)
		vector.StrokeRect(screen, x+0.5, y+0.5, cw-1, ch-1, 1, color.RGBA{12, 14, 15, 255}, false)
		if i == g.hover {
			vector.StrokeRect(screen, x+2, y+2, cw-4, ch-4, 2, color.RGBA{150, 170, 160, 255}, false)
		}
		label := fmt.Sprintf("%s  %d", c.g.Describe(), c.g.Mass)
		if c.starred {
			label = "* " + label
		}
		ebitenutil.DebugPrintAt(screen, label, int(x)+8, int(y+ch)-20)
	}
	hint := "click: variations of this one   right-click: star it   space: all new   F2 room  F3 desktop"
	if g.gen > 0 {
		hint = fmt.Sprintf("generation %d   ", g.gen) + hint
	}
	ebitenutil.DebugPrintAt(screen, hint, int(ox)+8, int(oy)+6)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%d starred (your pets hatch from these)", len(g.stars)), int(ox)+8, int(oy)+22)
}

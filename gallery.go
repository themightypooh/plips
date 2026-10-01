package main

import (
	"fmt"
	"image/color"
	"math/rand"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"plips/sim"
)

// The gallery shows a grid of baby plips with no brains, for finding a look.
const (
	cellW, cellH, cellFloor = 108, 80, 70
	gCols, gRows            = 4, 3
	galleryW, galleryH      = cellW * gCols, cellH * gRows
	nCells                  = gCols * gRows
)

type cell struct {
	g sim.Genome
	w *sim.World
	r *sim.Renderer
}

type Gallery struct {
	cells    []*cell
	batch    []*cell // the working batch, kept while looking at starred ones
	starView bool
	selected int
	hover    int
	gen      int
	stars    []sim.Genome

	bg, buf, canvas []byte
	img             *ebiten.Image
	rng             *rand.Rand
}

func NewGallery(rng *rand.Rand) *Gallery {
	g := &Gallery{
		bg:       sim.CellBackground(cellW, cellH, cellFloor),
		buf:      make([]byte, cellW*cellH*4),
		canvas:   make([]byte, galleryW*galleryH*4),
		img:      ebiten.NewImage(galleryW, galleryH),
		rng:      rng,
		selected: -1,
		hover:    -1,
	}
	load("favourites.json", &g.stars)
	g.NewBatch()
	return g
}

func (g *Gallery) makeCell(gn sim.Genome) *cell {
	w := sim.NewWorld(cellW, cellH, cellFloor, g.rng.Int63())
	w.AddPlip(sim.NewPlip(gn, "", false), cellW/2, nil)
	for k := 0; k < 120; k++ {
		w.Step()
	}
	return &cell{g: gn, w: w, r: sim.NewRenderer(cellW, cellH)}
}

func (g *Gallery) NewBatch() {
	g.batch = g.batch[:0]
	for i := 0; i < nCells; i++ {
		g.batch = append(g.batch, g.makeCell(sim.RandomGenome(g.rng)))
	}
	g.gen = 0
	g.ShowStarred(false)
}

// Breed refills the batch with variations of the selected plip.
func (g *Gallery) Breed() {
	parent, ok := g.Selected()
	if !ok {
		return
	}
	g.batch = []*cell{g.makeCell(parent)}
	for len(g.batch) < nCells {
		g.batch = append(g.batch, g.makeCell(parent.Mutate(g.rng, 0.55)))
	}
	g.gen++
	g.ShowStarred(false)
	g.selected = 0
}

func (g *Gallery) ShowStarred(on bool) {
	g.starView = on
	g.selected = -1
	if !on {
		g.cells = g.batch
		return
	}
	g.cells = nil
	for i := len(g.stars) - 1; i >= 0 && len(g.cells) < nCells; i-- {
		g.cells = append(g.cells, g.makeCell(g.stars[i]))
	}
}

func (g *Gallery) Selected() (sim.Genome, bool) {
	if g.selected < 0 || g.selected >= len(g.cells) {
		return sim.Genome{}, false
	}
	return g.cells[g.selected].g, true
}

func (g *Gallery) IsStarred(gn sim.Genome) bool {
	for _, s := range g.stars {
		if s.Instinct == gn.Instinct {
			return true
		}
	}
	return false
}

func (g *Gallery) ToggleStar() {
	gn, ok := g.Selected()
	if !ok {
		return
	}
	if g.IsStarred(gn) {
		for k, s := range g.stars {
			if s.Instinct == gn.Instinct {
				g.stars = append(g.stars[:k], g.stars[k+1:]...)
				break
			}
		}
	} else {
		g.stars = append(g.stars, gn)
	}
	save("favourites.json", g.stars)
}

// Update steps the cells; (sx, sy) is the cursor in gallery pixels.
func (g *Gallery) Update(sx, sy float32, inside, click bool) {
	g.hover = -1
	if inside {
		i := int(sy)/cellH*gCols + int(sx)/cellW
		if i < len(g.cells) {
			g.hover = i
			if click {
				g.selected = i
			}
		}
	}
	for _, c := range g.cells {
		c.w.Step()
		c.w.Step()
	}
}

func (g *Gallery) Draw(screen *ebiten.Image, ox, oy, s float64) {
	for i := range g.canvas {
		g.canvas[i] = 0
	}
	for y := 0; y < galleryH; y++ {
		for x := 0; x < galleryW; x++ {
			p := (y*galleryW + x) * 4
			g.canvas[p], g.canvas[p+1], g.canvas[p+2], g.canvas[p+3] = 14, 16, 17, 255
		}
	}
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
		vector.StrokeRect(screen, x+0.5, y+0.5, cw-1, ch-1, 1, color.RGBA{10, 12, 13, 255}, false)
		switch {
		case i == g.selected:
			vector.StrokeRect(screen, x+2, y+2, cw-4, ch-4, 3, colAccent, false)
		case i == g.hover:
			vector.StrokeRect(screen, x+2, y+2, cw-4, ch-4, 1, colDim, false)
		}
		label := c.g.Describe()
		if g.IsStarred(c.g) {
			label = "* " + label
		}
		drawText(screen, label, float64(x)+10, float64(y+ch)-24, 13, colDim)
	}
	if g.starView && len(g.cells) == 0 {
		drawText(screen, "Nothing starred yet. Go back to the batch, select a plip you like and press Star.", ox+20, oy+20, 15, colDim)
	}
}

// Caption is the status line for the bottom bar.
func (g *Gallery) Caption() string {
	switch {
	case g.starView:
		return fmt.Sprintf("%d starred", len(g.stars))
	case g.gen > 0:
		return fmt.Sprintf("generation %d", g.gen)
	}
	return "fresh batch"
}

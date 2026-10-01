// Command shot runs the simulation headless and writes PNGs, so the look and
// behaviour can be checked without a window (handy on Linux/CI).
//
//	go run ./cmd/shot -out shots
package main

import (
	"flag"
	"fmt"
	"image"
	"image/png"
	"math/rand"
	"os"
	"path/filepath"

	"plips/sim"
)

func main() {
	out := flag.String("out", "shots", "output folder")
	seed := flag.Int64("seed", 1, "random seed")
	secs := flag.Int("secs", 180, "seconds of room life to simulate")
	flag.Parse()
	os.MkdirAll(*out, 0o755)
	gallery(*out, *seed)
	room(*out, *seed, *secs)
	legs(*out, *seed)
}

const cellW, cellH, cellFloor, cols, rows = 108, 80, 70, 4, 3

func gallery(out string, seed int64) {
	r := rand.New(rand.NewSource(seed))
	W, H := cellW*cols, cellH*rows
	img := image.NewRGBA(image.Rect(0, 0, W, H))
	bg := sim.CellBackground(cellW, cellH, cellFloor)
	buf := make([]byte, cellW*cellH*4)
	base := sim.RandomGenome(r)
	for k := 0; k < cols*rows; k++ {
		g := sim.RandomGenome(r)
		if seed < 0 { // negative seed: show a family of mutants instead
			g = base.Mutate(r, 0.6)
		}
		w := sim.NewWorld(cellW, cellH, cellFloor, r.Int63())
		w.AddPlip(sim.NewPlip(g, "", false), cellW/2, nil)
		for i := 0; i < 600; i++ {
			w.Step()
		}
		rd := sim.NewRenderer(cellW, cellH)
		rd.Render(w, buf, bg)
		ox, oy := (k%cols)*cellW, (k/cols)*cellH
		for y := 0; y < cellH; y++ {
			copy(img.Pix[(oy+y)*img.Stride+ox*4:], buf[y*cellW*4:(y+1)*cellW*4])
		}
		fmt.Printf("cell %2d: %-32s mass %d\n", k, g.Describe(), w.Plips[0].Mass)
	}
	save(filepath.Join(out, "gallery.png"), img, 3)
}

func room(out string, seed int64, secs int) {
	r := rand.New(rand.NewSource(seed + 99))
	w := sim.NewWorld(sim.RoomW, sim.RoomH, sim.RoomFloor, seed)
	w.Platforms = sim.RoomPlatforms
	for i, x := range []float32{110, 210} {
		p := sim.NewPlip(sim.RandomGenome(r), sim.RandomName(r), true)
		_ = i
		w.AddPlip(p, x, nil)
	}
	bg := sim.RoomBackground()
	rd := sim.NewRenderer(sim.RoomW, sim.RoomH)
	buf := make([]byte, sim.RoomW*sim.RoomH*4)
	acts := map[string]int{}
	steps := secs * 120
	for s := 0; s < steps; s++ {
		if s%(120*6) == 0 { // somebody drops food every 6 s
			w.Splash(20+r.Float32()*280, 30, r.Intn(sim.NCol), 30)
		}
		if s%(120*20) == 0 && s > 0 { // and pets whoever is eating blue
			for _, p := range w.Plips {
				if p.Act == sim.AEat && p.Tgt == 4 {
					p.Pet()
				}
			}
		}
		w.Step()
		if s%120 == 0 {
			for _, p := range w.Plips {
				acts[p.Name+": "+sim.ActNames[p.Act]]++
			}
		}
		if s == steps/2 {
			rd.Render(w, buf, bg)
			saveBuf(filepath.Join(out, "room-mid.png"), buf, 4)
		}
	}
	rd.Render(w, buf, bg)
	saveBuf(filepath.Join(out, "room.png"), buf, 4)
	fmt.Println("particles:", w.N)
	for _, p := range w.Plips {
		fmt.Printf("%s mass %d drives %.2f doing %q likes:", p.Name, p.Mass, p.Drives, p.Doing(w))
		for c := 0; c < sim.NCol; c++ {
			fmt.Printf(" %s %.2f", sim.Colours[c].Name, p.Brain.Liking(c))
		}
		fmt.Println()
	}
	fmt.Println(acts)
}

func saveBuf(path string, buf []byte, scale int) {
	img := image.NewRGBA(image.Rect(0, 0, sim.RoomW, sim.RoomH))
	copy(img.Pix, buf)
	save(path, img, scale)
}

func save(path string, src *image.RGBA, scale int) {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx()*scale, b.Dy()*scale))
	for y := 0; y < dst.Bounds().Dy(); y++ {
		for x := 0; x < dst.Bounds().Dx(); x++ {
			si := (y/scale)*src.Stride + (x/scale)*4
			di := y*dst.Stride + x*4
			copy(dst.Pix[di:di+4], src.Pix[si:si+4])
		}
	}
	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	png.Encode(f, dst)
}

// legs renders one legged plip as its legs grow in and it learns to walk:
// a strip of frames, top row untrained, bottom row after some practice.
func legs(out string, seed int64) {
	r := rand.New(rand.NewSource(seed + 7))
	g := sim.RandomGenome(r)
	g.Limbs = []sim.Limb{{Kind: sim.LimbLeg, Len: 7, Stiff: 0.6, Tip: true, Mirror: true}}
	g.Hue, g.Sat, g.Val = 0.55, 0.35, 0.85 // light blue
	const fw, fh, fl, nf = 120, 56, 48, 6
	img := image.NewRGBA(image.Rect(0, 0, fw*nf, fh*2))
	bg := sim.CellBackground(fw, fh, fl)
	buf := make([]byte, fw*fh*4)
	rd := sim.NewRenderer(fw, fh)
	w := sim.NewWorld(fw, fh, fl, seed)
	p := sim.NewPlip(g, "", false)
	w.AddPlip(p, fw/2, nil)
	shot := func(row, col int) {
		rd.Render(w, buf, bg)
		for y := 0; y < fh; y++ {
			copy(img.Pix[(row*fh+y)*img.Stride+col*fw*4:], buf[y*fw*4:(y+1)*fw*4])
		}
	}
	tgt := float32(14)
	run := func(n int) {
		for i := 0; i < n; i++ {
			if d := p.CoreX - tgt; d*d < 16 {
				tgt = fw - tgt
			}
			p.TX = tgt
			w.Step()
		}
	}
	for f := 0; f < nf; f++ {
		run(9)
		shot(0, f)
	}
	run(120 * 60 * 5)
	for f := 0; f < nf; f++ {
		run(9)
		shot(1, f)
	}
	fmt.Printf("legs: %s, skill %.3f px/tick\n", p.WalkWords(), p.Motor.Skill)
	save(filepath.Join(out, "legs.png"), img, 4)
}

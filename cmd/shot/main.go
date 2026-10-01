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
	w.Objects = []*sim.Object{sim.NewBox(70), sim.NewBall(240)}
	p := sim.NewPlip(sim.RandomGenome(r), sim.RandomName(r), true)
	w.AddPlip(p, 160, nil)
	bg := sim.RoomBackground()
	rd := sim.NewRenderer(sim.RoomW, sim.RoomH)
	buf := make([]byte, sim.RoomW*sim.RoomH*4)
	steps := secs * 120
	type tally struct{ push, pushHungry, hungryDecisions int }
	var per []tally
	cur := tally{}
	lastAct := -1
	for s := 0; s < steps; s++ {
		w.Step()
		if p.Doing(w) != "" && (p.Act != lastAct || s%120 == 0) {
			lastAct = p.Act
		}
		if s%240 == 0 { // sample what it's up to every 2 s
			if p.Drives[sim.Hunger] > 0.4 {
				cur.hungryDecisions++
				if p.Act == sim.APush && p.Tgt == sim.TBox {
					cur.pushHungry++
				}
			}
			if p.Act == sim.APush && p.Tgt == sim.TBox {
				cur.push++
			}
		}
		if s%(120*120) == 120*120-1 {
			per = append(per, cur)
			cur = tally{}
		}
		if s == steps/2 {
			rd.Render(w, buf, bg)
			saveBuf(filepath.Join(out, "room-mid.png"), buf, 4)
		}
	}
	rd.Render(w, buf, bg)
	saveBuf(filepath.Join(out, "room.png"), buf, 4)
	fmt.Printf("%s: mass %d drives %.2f, box food %d, eaten %d, box dispensed %d\n", p.Name, p.Mass, p.Drives, p.BoxFood, p.Eaten, w.Objects[0].Dispensed)
	for i, t := range per {
		fmt.Printf("  minutes %2d-%2d: pushing box %2d/60 samples, when hungry %d/%d\n", i*2, i*2+2, t.push, t.pushHungry, t.hungryDecisions)
	}
	fmt.Println("ideas now:", p.Ideas(w, 4))
	for t := 0; t < sim.NT; t++ {
		for a := 0; a < sim.NA; a++ {
			if st := p.Stats[t][a]; st.N > 0 {
				fmt.Printf("    %-26s n=%3d reward %+.3f surprise %.3f\n", p.ActText(a, t), st.N, st.Reward/float32(st.N), st.Surprise/float32(st.N))
			}
		}
	}
	for _, d := range p.Diary {
		fmt.Println("  diary:", d.Text)
	}
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

// Package sim is the liquid-pixel world: particles, the room, and the critters
// living in it. It has no graphics dependencies so it can run headless.
package sim

import (
	"math"
	"math/rand"
)

const (
	W     = 320
	H     = 180
	Floor = 163 // y of the floor surface

	WallL = 3
	WallR = W - 3

	MaxP = 6000

	// Double-density relaxation (Clavet et al. 2005), tuned in pixel units.
	R    = 3.6
	R2   = R * R
	RHO0 = 3.2
	K    = 0.06
	KN   = 0.3
	G    = 0.045

	Dens = 0.62 // particles per px² at rest, used to size bodies

	EvapAge = 120 * 90 // loose liquid dries up after ~90 s
)

// Material is a kind of liquid you can drop. Index 0 is body plasm and is
// never dropped by the player.
type Material struct {
	Name   string
	C      [3]float32
	Hunger float32 // change to hunger per particle eaten
	Pain   float32 // change to pain per particle eaten
}

var Mats = []Material{
	{"Plasm", [3]float32{140, 150, 145}, 0, 0},
	{"Ink", [3]float32{52, 82, 140}, -0.010, 0},
	{"Rust", [3]float32{158, 74, 48}, -0.008, 0.012},
	{"Pollen", [3]float32{205, 162, 62}, -0.020, 0},
	{"Moss", [3]float32{84, 122, 62}, -0.015, 0},
	{"Milk", [3]float32{226, 221, 208}, -0.030, 0},
	{"Tar", [3]float32{38, 34, 32}, -0.004, 0.045},
}

// Platform is a one-way surface: liquid landing from above rests on it.
type Platform struct{ X0, X1, Y float32 }

var Platforms = []Platform{
	{20, 84, 104},   // wall shelf
	{228, 292, 128}, // table top
}

type World struct {
	X, Y, PX, PY, VX, VY []float32
	CR, CG, CB           []float32
	Own                  []uint8 // 0 = loose, otherwise critter ID
	Mat                  []uint8
	Age                  []int32
	N                    int

	Critters []*Critter

	Hand struct {
		X, Y    float32
		Visible bool
	}

	Rng  *rand.Rand
	Tick int

	head, next []int32
	gw, gh     int
	nb         [256]int32
	nq         [256]float32
}

func NewWorld(seed int64) *World {
	w := &World{Rng: rand.New(rand.NewSource(seed))}
	alloc := func() []float32 { return make([]float32, MaxP) }
	w.X, w.Y, w.PX, w.PY, w.VX, w.VY = alloc(), alloc(), alloc(), alloc(), alloc(), alloc()
	w.CR, w.CG, w.CB = alloc(), alloc(), alloc()
	w.Own = make([]uint8, MaxP)
	w.Mat = make([]uint8, MaxP)
	w.Age = make([]int32, MaxP)
	w.gw, w.gh = int(W/R)+2, int(H/R)+2
	w.head = make([]int32, w.gw*w.gh)
	w.next = make([]int32, MaxP)
	w.Reset()
	return w
}

// Reset clears the room and hatches a fresh pair.
func (w *World) Reset() {
	w.N = 0
	w.Critters = nil
	a := NewCritter(1, RandomGenome(w.Rng), w.Rng)
	b := NewCritter(2, RandomGenome(w.Rng), w.Rng)
	w.Critters = append(w.Critters, a, b)
	a.CoreX, b.CoreX = 110, 200
	for _, c := range w.Critters {
		c.TX = c.CoreX
		r := float32(math.Sqrt(float64(c.G.StartMass) / (math.Pi * Dens)))
		w.blob(c.CoreX, Floor-r, r, c.G.StartMass, 0, uint8(c.ID), c.G.Base)
	}
	// a little something to find
	w.Blob(160, Floor-3, 3, 14, 5)
	w.Blob(60, 40, 2.5, 12, 3)
	for i := 0; i < 200; i++ {
		w.Step()
	}
}

func (w *World) add(x, y float32, mat, own uint8, col [3]float32, vx, vy float32) {
	if w.N >= MaxP {
		return
	}
	i := w.N
	j := (w.Rng.Float32() - 0.5) * 16
	w.X[i], w.Y[i], w.VX[i], w.VY[i] = x, y, vx, vy
	w.CR[i], w.CG[i], w.CB[i] = col[0]+j, col[1]+j, col[2]+j
	w.Own[i], w.Mat[i], w.Age[i] = own, mat, 0
	w.N++
}

func (w *World) blob(cx, cy, r float32, count int, mat, own uint8, col [3]float32) {
	for k := 0; k < count; k++ {
		a := w.Rng.Float64() * 2 * math.Pi
		d := float32(math.Sqrt(w.Rng.Float64())) * r
		w.add(cx+float32(math.Cos(a))*d, cy+float32(math.Sin(a))*d, mat, own, col, 0, 0.2)
	}
}

// Blob drops a splash of loose liquid.
func (w *World) Blob(x, y, r float32, count int, mat uint8) {
	w.blob(x, y, r, count, mat, 0, Mats[mat].C)
}

// Drip adds one loose particle with a velocity (used for drops and streams).
func (w *World) Drip(x, y float32, mat uint8, vx, vy float32) {
	w.add(x, y, mat, 0, Mats[mat].C, vx, vy)
}

// Full reports whether the room has run out of particle budget.
func (w *World) Full() bool { return w.N > MaxP-64 }

func (w *World) remove(i int) {
	last := w.N - 1
	w.X[i], w.Y[i], w.PX[i], w.PY[i] = w.X[last], w.Y[last], w.PX[last], w.PY[last]
	w.VX[i], w.VY[i] = w.VX[last], w.VY[last]
	w.CR[i], w.CG[i], w.CB[i] = w.CR[last], w.CG[last], w.CB[last]
	w.Own[i], w.Mat[i], w.Age[i] = w.Own[last], w.Mat[last], w.Age[last]
	w.N--
}

// CritterAt returns the critter whose body is under (x, y), if any.
func (w *World) CritterAt(x, y float32) *Critter {
	for _, c := range w.Critters {
		if c.Mass > 20 && hypot(x-c.MX, y-c.MY) < c.Rad*1.15 {
			return c
		}
	}
	return nil
}

func (w *World) Other(c *Critter) *Critter {
	for _, o := range w.Critters {
		if o != c {
			return o
		}
	}
	return nil
}

// Step advances the world by one physics tick (the game runs two per frame).
func (w *World) Step() {
	w.Tick++
	w.census()
	for _, c := range w.Critters {
		c.Update(w)
	}
	w.housekeeping()

	for i := 0; i < w.N; i++ {
		w.VY[i] += G
		if o := w.Own[i]; o != 0 {
			c := w.Critters[o-1]
			dx, dy := c.CoreX-w.X[i], c.CoreY-w.Y[i]
			d := float32(math.Sqrt(float64(dx*dx+dy*dy))) + 1e-4
			w.VX[i] += dx * 0.0035
			w.VY[i] += dy * 0.0035
			if d > c.Rad {
				f := (d - c.Rad) * 0.012 / d
				w.VX[i] += dx * f
				w.VY[i] += dy * f
			}
		} else {
			w.Age[i]++
		}
		w.VX[i] *= 0.996
		w.VY[i] *= 0.996
		if sp := w.VX[i]*w.VX[i] + w.VY[i]*w.VY[i]; sp > 9 {
			s := 3 / float32(math.Sqrt(float64(sp)))
			w.VX[i] *= s
			w.VY[i] *= s
		}
		w.PX[i], w.PY[i] = w.X[i], w.Y[i]
		w.X[i] += w.VX[i]
		w.Y[i] += w.VY[i]
		w.collide(i)
	}
	w.grid()
	w.relax()
	for i := 0; i < w.N; i++ {
		w.collide(i)
		w.VX[i] = w.X[i] - w.PX[i]
		w.VY[i] = w.Y[i] - w.PY[i]
	}
}

func (w *World) collide(i int) {
	if w.X[i] < WallL {
		w.X[i] = WallL
	} else if w.X[i] > WallR {
		w.X[i] = WallR
	}
	if w.Y[i] < 1 {
		w.Y[i] = 1
	}
	if w.Y[i] >= Floor {
		w.Y[i] = Floor
		w.X[i] = w.X[i]*0.7 + w.PX[i]*0.3
		return
	}
	for _, p := range Platforms {
		if w.PY[i] <= p.Y && w.Y[i] > p.Y && w.X[i] >= p.X0 && w.X[i] <= p.X1 {
			w.Y[i] = p.Y
			w.X[i] = w.X[i]*0.7 + w.PX[i]*0.3
		}
	}
}

// census recounts each critter's mass and centre.
func (w *World) census() {
	type acc struct {
		sx, sy float32
		n      int
	}
	var a [8]acc
	for i := 0; i < w.N; i++ {
		if o := w.Own[i]; o != 0 {
			a[o].sx += w.X[i]
			a[o].sy += w.Y[i]
			a[o].n++
		}
	}
	for _, c := range w.Critters {
		s := a[c.ID]
		c.Mass = s.n
		if s.n > 0 {
			c.MX, c.MY = s.sx/float32(s.n), s.sy/float32(s.n)
		}
		c.Rad = float32(math.Sqrt(float64(c.Mass) / (math.Pi * Dens)))
	}
}

// housekeeping: starving critters waste away, old puddles dry up.
func (w *World) housekeeping() {
	for _, c := range w.Critters {
		if c.Drives[Hunger] > 0.85 && c.Mass > 50 && w.Tick%240 == 0 {
			for i := 0; i < w.N; i++ {
				if w.Own[i] == uint8(c.ID) {
					w.remove(i)
					break
				}
			}
		}
	}
	if w.N > 0 {
		i := w.Rng.Intn(w.N)
		if w.Own[i] == 0 && (w.Age[i] > EvapAge || (w.Mat[i] == 0 && w.Age[i] > EvapAge/4)) {
			w.remove(i)
		}
	}
}

func (w *World) grid() {
	for i := range w.head {
		w.head[i] = -1
	}
	for i := 0; i < w.N; i++ {
		c := int(w.Y[i]/R)*w.gw + int(w.X[i]/R)
		w.next[i] = w.head[c]
		w.head[c] = int32(i)
	}
}

func (w *World) relax() {
	for i := 0; i < w.N; i++ {
		xi, yi := w.X[i], w.Y[i]
		cx, cy := int(xi/R), int(yi/R)
		var rho, rhoN float32
		m := 0
		for gy := cy - 1; gy <= cy+1; gy++ {
			if gy < 0 || gy >= w.gh {
				continue
			}
			for gx := cx - 1; gx <= cx+1; gx++ {
				if gx < 0 || gx >= w.gw {
					continue
				}
				for j := w.head[gy*w.gw+gx]; j != -1; j = w.next[j] {
					if int(j) == i {
						continue
					}
					dx, dy := w.X[j]-xi, w.Y[j]-yi
					d2 := dx*dx + dy*dy
					if d2 < R2 && m < len(w.nb) {
						q := 1 - float32(math.Sqrt(float64(d2)))/R
						rho += q * q
						rhoN += q * q * q
						w.nb[m], w.nq[m] = j, q
						m++
					}
				}
			}
		}
		P, PN := K*(rho-RHO0), KN*rhoN
		var ddx, ddy float32
		own := w.Own[i]
		var c *Critter
		if own != 0 {
			c = w.Critters[own-1]
		}
		for k := 0; k < m; k++ {
			j, q := w.nb[k], w.nq[k]
			dx, dy := w.X[j]-xi, w.Y[j]-yi
			d := float32(math.Sqrt(float64(dx*dx + dy*dy)))
			if d < 1e-3 {
				d = 1e-3
			}
			dx /= d
			dy /= d
			D := (P*q + PN*q*q) * 0.5
			w.X[j] += dx * D
			w.Y[j] += dy * D
			ddx -= dx * D
			ddy -= dy * D
			if c == nil {
				continue
			}
			oj := w.Own[j]
			switch {
			case oj == own:
				// slow colour bleed between neighbours
				w.CR[i] += (w.CR[j] - w.CR[i]) * 0.00025
				w.CG[i] += (w.CG[j] - w.CG[i]) * 0.00025
				w.CB[i] += (w.CB[j] - w.CB[i]) * 0.00025
			case oj == 0:
				if q > 0.45 && c.EatMat >= 0 && int(w.Mat[j]) == c.EatMat && c.Mass < c.MaxMass() {
					w.Own[j] = own
					c.Mass++
					c.Ate(w.Mat[j])
				}
			case oj == c.StealFrom && q > 0.5 && c.stealCD <= 0:
				o := w.Critters[oj-1]
				if o.Mass > 50 {
					w.Own[j] = own
					c.Mass++
					o.Mass--
					c.stealCD = 8
					c.Ate(w.Mat[j])
					o.Hurt(0.025)
				}
			}
		}
		w.X[i] += ddx
		w.Y[i] += ddy
	}
}

func hypot(x, y float32) float32 { return float32(math.Sqrt(float64(x*x + y*y))) }

func clamp01(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

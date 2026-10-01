// Package sim is the liquid-pixel world: particles, surfaces, and the plips
// living in it. It has no graphics dependencies so it can run headless.
package sim

import (
	"math"
	"math/rand"
)

const (
	MaxP = 6000

	// Double-density relaxation (Clavet et al. 2005), tuned in pixel units.
	R    = 3.6
	R2   = R * R
	RHO0 = 3.2
	K    = 0.06
	KN   = 0.3
	G    = 0.045

	Dens = 0.62 // particles per px² at rest, used to size bodies

	EvapAge = 120 * 120 // loose liquid dries up after ~2 min
	MinMass = 40
)

// Colour is a food colour you can drop. Particle material 0 is body plasm;
// materials 1..NCol are these colours.
type Colour struct {
	Name string
	C    [3]float32
}

const NCol = 8

var Colours = [NCol]Colour{
	{"Red", [3]float32{186, 62, 54}},
	{"Orange", [3]float32{212, 124, 54}},
	{"Yellow", [3]float32{220, 188, 72}},
	{"Green", [3]float32{82, 148, 82}},
	{"Blue", [3]float32{62, 102, 186}},
	{"Violet", [3]float32{128, 82, 168}},
	{"White", [3]float32{228, 226, 218}},
	{"Black", [3]float32{34, 33, 37}},
}

// Particle roles within a plip.
const (
	RoleBody = iota
	RoleNucleus
	RoleLimb
)

// Platform is a one-way surface: liquid landing from above rests on it.
type Platform struct{ X0, X1, Y float32 }

type World struct {
	W, H, Floor float32
	Platforms   []Platform

	X, Y, PX, PY, VX, VY []float32
	CR, CG, CB           []float32
	Own                  []uint8 // 0 = loose, otherwise plip ID
	Mat                  []uint8 // 0 = body plasm, 1..NCol = food colour
	Lobe, Role, Seg      []uint8 // body lobe (or limb index); role: see Role*; limb segment
	Age                  []int32
	N                    int

	Plips      []*Plip
	Objects    []*Object
	FoodEvents []FoodEvent // recent food appearances, newest last

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

func NewWorld(w, h, floor float32, seed int64) *World {
	wd := &World{W: w, H: h, Floor: floor, Rng: rand.New(rand.NewSource(seed))}
	alloc := func() []float32 { return make([]float32, MaxP) }
	wd.X, wd.Y, wd.PX, wd.PY, wd.VX, wd.VY = alloc(), alloc(), alloc(), alloc(), alloc(), alloc()
	wd.CR, wd.CG, wd.CB = alloc(), alloc(), alloc()
	wd.Own = make([]uint8, MaxP)
	wd.Mat = make([]uint8, MaxP)
	wd.Lobe = make([]uint8, MaxP)
	wd.Role = make([]uint8, MaxP)
	wd.Seg = make([]uint8, MaxP)
	wd.Age = make([]int32, MaxP)
	wd.gw, wd.gh = int(w/R)+2, int(h/R)+2
	wd.head = make([]int32, wd.gw*wd.gh)
	wd.next = make([]int32, MaxP)
	return wd
}

// Clear removes every particle and plip.
func (w *World) Clear() {
	w.N = 0
	w.Plips = nil
}

// AddPlip hatches a plip at x. colours, if given, are its saved body colours
// (one per particle); otherwise the genome decides.
func (w *World) AddPlip(p *Plip, x float32, colours [][3]uint8) {
	p.ID = len(w.Plips) + 1
	w.Plips = append(w.Plips, p)
	p.CoreX, p.TX = x, x
	mass := p.G.Mass
	if len(colours) > 0 {
		mass = len(colours)
	}
	p.Mass = mass
	p.layout(w)
	base, spot := p.G.BaseColor(), p.G.SpotColor()
	weights := p.lobeWeights()
	for k := 0; k < mass; k++ {
		l := pickWeighted(w.Rng, weights)
		role := uint8(0)
		if w.Rng.Float32() < p.G.Nucleus {
			role, l = 1, 0
		}
		col := base
		jit := (w.Rng.Float32() - 0.5) * p.G.Speckle
		if role == 1 {
			col = [3]float32{base[0] * 0.55, base[1] * 0.55, base[2] * 0.6}
		} else if w.Rng.Float32() < p.G.Spots {
			col = spot
		}
		col = [3]float32{col[0] + jit, col[1] + jit, col[2] + jit}
		if k < len(colours) {
			c := colours[k]
			col = [3]float32{float32(c[0]), float32(c[1]), float32(c[2])}
		}
		a := w.Rng.Float64() * 2 * math.Pi
		d := float32(math.Sqrt(w.Rng.Float64())) * p.LR[l] * 0.8
		w.addRaw(p.CoreX+p.LX[l]+float32(math.Cos(a))*d, p.CoreY+p.LY[l]+float32(math.Sin(a))*d,
			0, uint8(p.ID), l, role, col, 0, 0)
	}
	limbCol := [3]float32{base[0] * 0.82, base[1] * 0.82, base[2] * 0.85}
	tipCol := [3]float32{base[0]*1.1 + 12, base[1]*1.1 + 12, base[2]*1.1 + 12}
	for li, lm := range p.limbs {
		ax, ay := p.limbAnchor(li)
		for s := 0; s < lm.segs(); s++ {
			c := limbCol
			if s >= lm.Len {
				c = tipCol
			}
			w.addRaw(ax, ay-float32(s), 0, uint8(p.ID), uint8(li), RoleLimb, c, 0, 0)
			w.Seg[w.N-1] = uint8(s)
		}
	}
}

func pickWeighted(r *rand.Rand, w []float32) uint8 {
	var t float32
	for _, v := range w {
		t += v
	}
	x := r.Float32() * t
	for i, v := range w {
		if x < v {
			return uint8(i)
		}
		x -= v
	}
	return uint8(len(w) - 1)
}

func (w *World) addRaw(x, y float32, mat, own, lobe, role uint8, col [3]float32, vx, vy float32) {
	if w.N >= MaxP {
		return
	}
	i := w.N
	w.X[i], w.Y[i], w.PX[i], w.PY[i], w.VX[i], w.VY[i] = x, y, x, y, vx, vy
	w.CR[i], w.CG[i], w.CB[i] = col[0], col[1], col[2]
	w.Own[i], w.Mat[i], w.Lobe[i], w.Role[i], w.Age[i], w.Seg[i] = own, mat, lobe, role, 0, 0
	w.N++
}

func (w *World) loose(x, y float32, col int, vx, vy float32) {
	c := Colours[col].C
	j := (w.Rng.Float32() - 0.5) * 14
	w.addRaw(x, y, uint8(col+1), 0, 0, 0, [3]float32{c[0] + j, c[1] + j, c[2] + j}, vx, vy)
}

// Splash drops a blob of loose liquid of colour col (0..NCol-1).
func (w *World) Splash(x, y float32, col, count int) {
	w.FoodEvents = append(w.FoodEvents, FoodEvent{X: x, Tick: w.Tick})
	for k := 0; k < count; k++ {
		a := w.Rng.Float64() * 2 * math.Pi
		d := float32(math.Sqrt(w.Rng.Float64())) * 3.2
		w.loose(x+float32(math.Cos(a))*d, y+float32(math.Sin(a))*d, col, 0, 0.2)
	}
}

// Drop adds a single droplet (a few particles so it reads as one drop).
func (w *World) Drop(x, y float32, col int) {
	w.FoodEvents = append(w.FoodEvents, FoodEvent{X: x, Tick: w.Tick})
	for k := 0; k < 3; k++ {
		w.loose(x+(w.Rng.Float32()-0.5)*0.8, y+(w.Rng.Float32()-0.5)*0.8, col, 0, 0.3)
	}
}

// Stream adds one particle of a falling stream.
func (w *World) Stream(x, y float32, col int) {
	if n := len(w.FoodEvents); n == 0 || w.Tick-w.FoodEvents[n-1].Tick > 30 {
		w.FoodEvents = append(w.FoodEvents, FoodEvent{X: x, Tick: w.Tick})
	}
	w.loose(x+(w.Rng.Float32()-0.5)*0.3, y, col, 0, 1.4)
}

// Full reports whether the particle budget is nearly used up.
func (w *World) Full() bool { return w.N > MaxP-64 }

func (w *World) remove(i int) {
	last := w.N - 1
	w.X[i], w.Y[i], w.PX[i], w.PY[i] = w.X[last], w.Y[last], w.PX[last], w.PY[last]
	w.VX[i], w.VY[i] = w.VX[last], w.VY[last]
	w.CR[i], w.CG[i], w.CB[i] = w.CR[last], w.CG[last], w.CB[last]
	w.Own[i], w.Mat[i], w.Lobe[i], w.Role[i], w.Age[i], w.Seg[i] = w.Own[last], w.Mat[last], w.Lobe[last], w.Role[last], w.Age[last], w.Seg[last]
	w.N--
}

// PlipAt returns the plip under (x, y), if any.
func (w *World) PlipAt(x, y float32) *Plip {
	for _, p := range w.Plips {
		if p.Mass > 10 && hypot(x-p.MX, y-p.MY) < p.Rad*1.2+1 {
			return p
		}
	}
	return nil
}

// Other returns the nearest other plip, or nil if it's alone.
func (w *World) Other(p *Plip) *Plip {
	var best *Plip
	bd := float32(1e9)
	for _, o := range w.Plips {
		if o == p {
			continue
		}
		if d := hypot(o.MX-p.MX, o.MY-p.MY); d < bd {
			best, bd = o, d
		}
	}
	return best
}

// RemovePlip takes a plip out of the world, body and all.
func (w *World) RemovePlip(p *Plip) {
	id := uint8(p.ID)
	for i := 0; i < w.N; {
		if w.Own[i] == id {
			w.remove(i)
			continue
		}
		i++
	}
	w.Plips = append(w.Plips[:p.ID-1], w.Plips[p.ID:]...)
	for k, q := range w.Plips {
		q.ID = k + 1
		q.StealFrom = 0
	}
	for i := 0; i < w.N; i++ {
		if w.Own[i] > id {
			w.Own[i]--
		}
	}
}

// ClearLoose removes all spilled liquid.
func (w *World) ClearLoose() {
	for i := 0; i < w.N; {
		if w.Own[i] == 0 {
			w.remove(i)
			continue
		}
		i++
	}
}

// BodyColours returns a plip's body colours, one per particle, for saving.
func (w *World) BodyColours(p *Plip) [][3]uint8 {
	var out [][3]uint8
	for i := 0; i < w.N; i++ {
		if w.Own[i] == uint8(p.ID) && w.Role[i] != RoleLimb {
			out = append(out, [3]uint8{u8(w.CR[i]), u8(w.CG[i]), u8(w.CB[i])})
		}
	}
	return out
}

func u8(v float32) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

// Step advances the world by one physics tick (the game runs two per frame).
func (w *World) Step() {
	w.Tick++
	w.stepObjects()
	for len(w.FoodEvents) > 0 && w.Tick-w.FoodEvents[0].Tick > 1200 {
		w.FoodEvents = w.FoodEvents[1:]
	}
	w.census()
	for _, p := range w.Plips {
		p.Update(w)
	}
	w.housekeeping()

	for i := 0; i < w.N; i++ {
		damp := float32(0.996)
		if o := w.Own[i]; o == 0 {
			w.VY[i] += G
			w.Age[i]++
		} else {
			p := w.Plips[o-1]
			if w.Role[i] == RoleLimb {
				w.VY[i] += G * 0.3
				w.VX[i] *= 0.85
				w.VY[i] *= 0.85
				goto move
			}
			w.VY[i] += G * p.gScale // bodies are partly self-supporting
			l := w.Lobe[i]
			cx, cy, r := p.CoreX+p.LX[l], p.CoreY+p.LY[l], p.LR[l]*p.breathScale
			kout := p.kOut
			if w.Role[i] == RoleNucleus {
				r *= 0.38
				kout *= 2
			}
			dx, dy := cx-w.X[i], cy-w.Y[i]
			if p.G.Lumpy > 0 && w.Role[i] == RoleBody {
				th := math.Atan2(float64(-dy), float64(-dx))
				r *= 1 + p.G.Lumpy*0.3*float32(math.Sin(float64(p.G.LumpK)*th+p.lumpPh))
			}
			w.VX[i] += dx * p.kIn
			w.VY[i] += dy * p.kIn
			ex, ey := dx/(r*p.ax), dy/(r*p.ay)
			if e := float32(math.Sqrt(float64(ex*ex + ey*ey))); e > 1 {
				f := kout * (e - 1) / e
				w.VX[i] += dx * f
				w.VY[i] += dy * f
			}
			damp = p.damp
		}
		w.VX[i] *= damp
		w.VY[i] *= damp
	move:
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
	for _, p := range w.Plips {
		p.solveLimbs(w)
	}
	for i := 0; i < w.N; i++ {
		w.collide(i)
		w.VX[i] = w.X[i] - w.PX[i]
		w.VY[i] = w.Y[i] - w.PY[i]
	}
}

func (w *World) collide(i int) {
	if w.X[i] < 2 {
		w.X[i] = 2
	} else if w.X[i] > w.W-2 {
		w.X[i] = w.W - 2
	}
	if w.Y[i] < 1 {
		w.Y[i] = 1
	}
	if len(w.Objects) > 0 {
		w.pushOut(i)
	}
	if w.Y[i] >= w.Floor {
		w.Y[i] = w.Floor
		w.X[i] = w.X[i]*0.7 + w.PX[i]*0.3
		return
	}
	if w.Own[i] != 0 {
		return // plips walk under furniture; only loose liquid lands on it
	}
	for _, p := range w.Platforms {
		if w.PY[i] <= p.Y && w.Y[i] > p.Y && w.X[i] >= p.X0 && w.X[i] <= p.X1 {
			w.Y[i] = p.Y
			w.X[i] = w.X[i]*0.7 + w.PX[i]*0.3
		}
	}
}

// census recounts each plip's mass, centre and head position.
func (w *World) census() {
	type acc struct {
		sx, sy, hx, hy float32
		n, hn          int
	}
	var a [16]acc
	for _, p := range w.Plips {
		for li := range p.limbIdx {
			for sg := range p.limbIdx[li] {
				p.limbIdx[li][sg] = -1
			}
		}
	}
	for i := 0; i < w.N; i++ {
		if o := w.Own[i]; o != 0 {
			if w.Role[i] == RoleLimb {
				p := w.Plips[o-1]
				if li, sg := int(w.Lobe[i]), int(w.Seg[i]); li < len(p.limbIdx) && sg < len(p.limbIdx[li]) {
					p.limbIdx[li][sg] = int32(i)
				}
				continue
			}
			s := &a[o]
			s.sx += w.X[i]
			s.sy += w.Y[i]
			s.n++
			if w.Lobe[i] == 0 && w.Role[i] == 0 {
				s.hx += w.X[i]
				s.hy += w.Y[i]
				s.hn++
			}
		}
	}
	for _, p := range w.Plips {
		s := a[p.ID]
		p.Mass = s.n
		if s.n > 0 {
			p.MX, p.MY = s.sx/float32(s.n), s.sy/float32(s.n)
		}
		if s.hn > 0 {
			p.HeadX, p.HeadY = s.hx/float32(s.hn), s.hy/float32(s.hn)
		}
		p.layout(w)
	}
}

// housekeeping: starving plips waste away, old puddles dry up.
func (w *World) housekeeping() {
	for _, p := range w.Plips {
		if p.Drives[Hunger] > 0.9 && p.Mass > MinMass && w.Tick%360 == 0 {
			for i := 0; i < w.N; i++ {
				if w.Own[i] == uint8(p.ID) && w.Role[i] == 0 {
					w.remove(i)
					break
				}
			}
		}
	}
	for k := 0; k < 2 && w.N > 0; k++ {
		i := w.Rng.Intn(w.N)
		if w.Own[i] == 0 && w.Age[i] > EvapAge {
			w.remove(i)
		}
	}
}

func (w *World) grid() {
	for i := range w.head {
		w.head[i] = -1
	}
	for i := 0; i < w.N; i++ {
		if w.Role[i] == RoleLimb {
			continue
		}
		gx, gy := int(w.X[i]/R), int(w.Y[i]/R)
		if gx >= w.gw {
			gx = w.gw - 1
		}
		if gy >= w.gh {
			gy = w.gh - 1
		}
		c := gy*w.gw + gx
		w.next[i] = w.head[c]
		w.head[c] = int32(i)
	}
}

func (w *World) relax() {
	for i := 0; i < w.N; i++ {
		if w.Role[i] == RoleLimb {
			continue
		}
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
		var p *Plip
		if own != 0 {
			p = w.Plips[own-1]
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
			if p == nil {
				if w.Own[j] == 0 && w.Mat[j] == w.Mat[i] && q < 0.7 {
					// loose liquid of one colour clings to itself, so puddles bead
					c := 0.012 * q
					w.X[j] -= dx * c
					w.Y[j] -= dy * c
					ddx += dx * c
					ddy += dy * c
				}
				continue
			}
			oj := w.Own[j]
			switch {
			case oj == own:
				if w.Role[i] == w.Role[j] { // slow colour bleed = swirls
					w.CR[i] += (w.CR[j] - w.CR[i]) * 0.00025
					w.CG[i] += (w.CG[j] - w.CG[i]) * 0.00025
					w.CB[i] += (w.CB[j] - w.CB[i]) * 0.00025
				}
			case oj == 0:
				if w.Mat[j] == 0 && q > 0.5 && w.Age[j] > 60 {
					// knocked-loose body goo gets slurped back up on contact
					w.Own[j] = own
					w.Lobe[j] = w.Lobe[i]
					p.Mass++
				} else if q > 0.45 && p.EatCol >= 0 && int(w.Mat[j]) == p.EatCol+1 && p.Mass < p.MaxMass() {
					w.Own[j] = own
					w.Lobe[j] = w.Lobe[i]
					p.Mass++
					p.Ate(int(w.Mat[j]) - 1)
				}
			case oj == p.StealFrom && q > 0.5 && p.stealCD <= 0:
				o := w.Plips[oj-1]
				if o.Mass > MinMass && w.Role[j] == 0 {
					w.Own[j] = own
					w.Lobe[j] = w.Lobe[i]
					p.Mass++
					o.Mass--
					p.stealCD = 10
					p.Drives[Hunger] = clamp01(p.Drives[Hunger] - 0.012)
					o.Hurt(0.03)
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

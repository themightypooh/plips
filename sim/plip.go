package sim

import (
	"math"
	"math/rand"
)

// Plip is one creature: a body made of particles held around a moving core,
// plus (outside the gallery) a brain that decides where that core goes.
type Plip struct {
	ID    int
	Name  string
	G     Genome
	Brain *Brain // nil = no mind, just idles (gallery)

	CoreX, CoreY, TX float32

	// measured each tick from the particles
	Mass                 int
	MX, MY, HeadX, HeadY float32
	Rad                  float32

	// body shape, recomputed each tick from genome + mass
	LX, LY, LR                  [4]float32
	ax, ay, kIn, kOut, damp     float32
	breathScale, lift, wigglePh float32
	breath                      float32

	Face                 float32
	Hop                  int
	Blink, Flinch, Happy int
	Moving, Resting      bool

	Drives    [NDrive]float32
	EatCol    int   // colour being eaten right now, -1 = none
	StealFrom uint8 // plip ID being nibbled, 0 = none
	stealCD   int

	// current decision (see mind.go)
	Act, Tgt     int
	actT, actMax int
	x            []float32
	sit          Situation
	startBad     float32
	startHunger  float32
	startPain    float32
	startTick    int
	tgtX         float32
	tgtStartX    float32
	tgtOK        bool
	bumped       bool
	out          [NO]float32
	prevX        []float32
	prevR        float32
	recentSurp   [NT]float32
	petTeach     map[[2]int]int

	LastReward   float32
	LastSurprise float32
	SurpriseAt   int // target that last surprised it
	Expect       [NO]float32
	Diary        []DiaryEntry
	Seen         map[string]bool
	BoxFood      int
	lastNote     int
	favourite    int
	favAt        int
	Eaten        int
	Stats        [NT][NA]struct {
		N                int
		Reward, Surprise float32
	}

	wanderT int
	Age     int

	limbs   []limbInst
	limbIdx [][]int32 // particle index per limb segment, rebuilt each tick
	gScale  float32
	lumpPh  float64
	limbPh  float32
}

// limbInst is one physical limb (a mirrored gene makes two).
type limbInst struct {
	Limb
	side float32 // -1/+1 for a mirrored pair, 0 otherwise
}

func (l limbInst) segs() int {
	if l.Tip {
		return l.Len + 2
	}
	return l.Len
}

func NewPlip(g Genome, name string, withBrain bool) *Plip {
	p := &Plip{G: g, Name: name, Face: 1, EatCol: -1}
	p.lumpPh = float64(g.Instinct%628) / 100
	for _, l := range g.Limbs {
		if l.Len < 1 {
			continue
		}
		if l.Mirror {
			p.limbs = append(p.limbs, limbInst{l, -1}, limbInst{l, 1})
		} else {
			p.limbs = append(p.limbs, limbInst{l, 0})
		}
	}
	for _, l := range p.limbs {
		idx := make([]int32, l.segs())
		for i := range idx {
			idx[i] = -1
		}
		p.limbIdx = append(p.limbIdx, idx)
	}
	if withBrain {
		p.Brain = NewBrain(g)
		p.Drives = [NDrive]float32{0.3, 0, 0.3, 0.2, 0.1}
		p.Seen = map[string]bool{}
		p.favourite = -1
	}
	return p
}

func (p *Plip) MaxMass() int {
	m := int(float32(p.G.Mass) * 2.2)
	if m > 330 {
		m = 330
	}
	return m
}

func (p *Plip) lobes() int {
	if p.G.Lobes < 1 {
		return 1
	}
	if p.G.Lobes > 4 {
		return 4
	}
	return p.G.Lobes
}

func (p *Plip) lobeWeights() []float32 {
	n := p.lobes()
	w := make([]float32, n)
	v := float32(1)
	for i := range w {
		w[i] = v
		v *= p.G.LobeTaper
	}
	if p.G.Layout == LayoutStack || p.G.Layout == LayoutClump {
		// the head (lobe 0) sits on top and is the smallest
		for i, j := 0, n-1; i < j; i, j = i+1, j-1 {
			w[i], w[j] = w[j], w[i]
		}
	}
	return w
}

// layout recomputes the lobe targets and body springs.
func (p *Plip) layout(w *World) {
	g := p.G
	total := float32(math.Sqrt(float64(p.Mass)/(math.Pi*Dens))) * g.Pack
	p.Rad = total
	p.ax = float32(math.Sqrt(float64(g.Aspect)))
	p.ay = 1 / p.ax
	p.kIn = lerp(0.0025, 0.012, g.Soft)
	p.kOut = lerp(0.01, 0.06, g.Taut)
	p.damp = lerp(0.984, 0.998, g.Jiggle)
	p.gScale = lerp(0.12, 0.7, g.Sag)
	p.breathScale = 1 + g.Breath*0.1*float32(math.Sin(float64(p.breath)))

	ws := p.lobeWeights()
	var sum float32
	for _, v := range ws {
		sum += v
	}
	n := len(ws)
	for i := range p.LR {
		p.LX[i], p.LY[i], p.LR[i] = 0, 0, 0
	}
	for i, v := range ws {
		p.LR[i] = total * float32(math.Sqrt(float64(v/sum)))
	}
	gap := g.LobeGap
	switch g.Layout {
	case LayoutChain:
		x := float32(0)
		for i := 1; i < n; i++ {
			x -= (p.LR[i-1] + p.LR[i]) * p.ax * gap
			p.LX[i] = x
		}
		mid := x / 2
		for i := 0; i < n; i++ {
			p.LX[i] = (p.LX[i] - mid) * p.Face
			if p.Moving {
				p.LY[i] = float32(math.Sin(float64(p.wigglePh-float32(i)*1.3))) * p.LR[i] * 0.15
			}
		}
	case LayoutStack:
		y := float32(0)
		for i := n - 2; i >= 0; i-- {
			y -= (p.LR[i+1] + p.LR[i]) * p.ay * gap
			p.LY[i] = y
		}
		if p.Moving {
			p.LX[0] = float32(math.Sin(float64(p.wigglePh))) * p.LR[0] * 0.15
		}
	case LayoutClump:
		if n == 2 {
			p.LX[0] = p.Face * p.LR[1] * 0.4
			p.LY[0] = -(p.LR[0] + p.LR[1]) * p.ay * gap * 0.8
		} else {
			p.LX[1] = -p.LR[1] * p.ax * gap * 0.9
			p.LX[2] = p.LR[2] * p.ax * gap * 0.9
			p.LY[0] = -(p.LR[0] + p.LR[1]) * p.ay * gap * 0.85
			if n == 4 {
				p.LX[3] = p.Face * p.LR[3] * 1.4
				p.LY[3] = -p.LR[3] * 0.6
			}
		}
	}
	// stand the lowest lobe on the floor
	low := 0
	for i := 1; i < n; i++ {
		if p.LY[i]+p.LR[i]*p.ay > p.LY[low]+p.LR[low]*p.ay {
			low = i
		}
	}
	p.CoreY = w.Floor - p.LY[low] - p.LR[low]*p.ay*0.85*p.breathScale - p.lift
}

// Update runs once per tick: body animation, then the mind (or idling).
func (p *Plip) Update(w *World) {
	p.Age++
	if p.Blink > 0 {
		p.Blink--
	} else if w.Rng.Float32() < 0.004 {
		p.Blink = 8
	}
	if p.Flinch > 0 {
		p.Flinch--
	}
	if p.Happy > 0 {
		p.Happy--
	}
	if p.stealCD > 0 {
		p.stealCD--
	}
	p.breath += 0.025 + p.G.Breath*0.02
	p.limbPh += 0.05
	p.EatCol, p.StealFrom, p.Resting = -1, 0, false

	if p.Brain != nil {
		p.drift(w)
		p.think(w)
	} else {
		p.idle(w)
	}

	// walk toward TX
	lo, hi := p.Rad+3, w.W-p.Rad-3
	if p.TX < lo {
		p.TX = lo
	} else if p.TX > hi {
		p.TX = hi
	}
	dx := p.TX - p.CoreX
	speed := 0.2 * p.G.Speed
	if math.Abs(float64(dx)) > 0.6 {
		p.Face = sign(dx)
	}
	step := clamp(dx*0.03, -speed, speed)
	p.CoreX += step
	p.Moving = math.Abs(float64(dx)) > 1.5
	if p.Moving {
		p.wigglePh += 0.12 * p.G.Speed
	}

	p.lift = 0
	if p.Hop > 0 {
		p.Hop++
		t := float32(p.Hop) / 46
		p.lift = float32(math.Sin(math.Min(float64(t), 1)*math.Pi)) * p.Rad * (0.45 + p.G.Hoppy*0.6)
		if t >= 1 {
			p.Hop = 0
		}
	}
}

func (p *Plip) idle(w *World) {
	p.wanderT--
	if p.wanderT <= 0 {
		p.TX = p.Rad + 3 + w.Rng.Float32()*(w.W-2*p.Rad-6)
		p.wanderT = 200 + w.Rng.Intn(400)
		if w.Rng.Float32() < p.G.Hoppy*0.6 && p.Hop == 0 {
			p.Hop = 1
		}
	}
}

// RandomName makes a short soft name.
func RandomName(r *rand.Rand) string {
	cons := []string{"b", "p", "m", "l", "n", "t", "d", "g", "w", "v", "z", "s", "f"}
	vows := []string{"a", "o", "i", "u", "e", "oo", "ee", "y"}
	ends := []string{"", "", "", "p", "b", "m", "n", "s", "x"}
	s := cons[r.Intn(len(cons))] + vows[r.Intn(len(vows))]
	if r.Intn(2) == 0 {
		s += cons[r.Intn(len(cons))] + vows[r.Intn(len(vows))]
	}
	s += ends[r.Intn(len(ends))]
	return string(s[0]-32) + s[1:]
}

func sign(v float32) float32 {
	if v < 0 {
		return -1
	}
	return 1
}

func clamp(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// limbDir is the resting direction a limb points, in screen space (y down).
func (p *Plip) limbDir(li int) (float32, float32) {
	l := p.limbs[li]
	var a float64
	switch l.Kind {
	case LimbFeeler:
		a = -math.Pi/2 + float64(l.side)*0.45 + float64(l.Angle)*0.6
	case LimbTail:
		a = math.Pi + 0.35 + float64(l.Angle)*0.5 // up-and-back, mirrored below
	case LimbWhisker:
		a = float64(l.side)*0.35 + float64(l.Angle)*0.4
	case LimbNub:
		a = 0.7 + float64(l.Angle)*0.3
	}
	a += math.Sin(float64(p.limbPh)+float64(li)*1.7) * 0.12 // idle sway
	dx, dy := float32(math.Cos(a)), float32(math.Sin(a))
	switch l.Kind {
	case LimbTail, LimbWhisker:
		dx *= p.Face // these follow the way it faces
	case LimbNub:
		if l.side < 0 {
			dx = -dx
		}
	}
	return dx, dy
}

// limbAnchor is where a limb joins the body.
func (p *Plip) limbAnchor(li int) (float32, float32) {
	l := p.limbs[li]
	lobe := 0
	n := p.lobes()
	if l.Kind == LimbTail || l.Kind == LimbNub {
		// the lobe furthest back, or the lowest one
		for i := 1; i < n; i++ {
			if l.Kind == LimbTail && p.LX[i]*p.Face < p.LX[lobe]*p.Face {
				lobe = i
			}
			if l.Kind == LimbNub && p.LY[i] > p.LY[lobe] {
				lobe = i
			}
		}
	}
	dx, dy := p.limbDir(li)
	if l.Kind == LimbNub {
		dy = 0.3
	}
	r := p.LR[lobe] * 0.85
	return p.CoreX + p.LX[lobe] + dx*r*p.ax, p.CoreY + p.LY[lobe] + dy*r*p.ay
}

// solveLimbs keeps each limb a chain of evenly spaced particles that leans
// toward its resting direction but swings and droops with motion.
func (p *Plip) solveLimbs(w *World) {
	for li, l := range p.limbs {
		px, py := p.limbAnchor(li)
		rx, ry := p.limbDir(li)
		k := 0.04 + l.Stiff*0.5
		droop := (1 - l.Stiff) * 0.35
		for s, i := range p.limbIdx[li] {
			if i < 0 {
				continue
			}
			seg := float32(1.15)
			if s >= l.Len {
				seg = 0.55 // tip blob
			}
			cx, cy := w.X[i]-px, w.Y[i]-py
			if d := hypot(cx, cy); d > 1e-3 {
				cx, cy = cx/d, cy/d
			} else {
				cx, cy = rx, ry
			}
			dx := rx*k + cx*(1-k)
			dy := ry*k + cy*(1-k) + droop*0.3
			if d := hypot(dx, dy); d > 1e-3 {
				dx, dy = dx/d, dy/d
			}
			w.X[i], w.Y[i] = px+dx*seg, py+dy*seg
			px, py = w.X[i], w.Y[i]
		}
	}
}

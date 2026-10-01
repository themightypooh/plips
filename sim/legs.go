package sim

import "math"

// Limbs grow in as a plip ages, one segment at a time, each made from a drop
// of its own body (so a well-fed plip grows faster). Legs are jointed: the
// motor net (motor.go) sets hip and knee angles, a foot touching the floor
// stays put, and the body is carried along by whatever the planted feet do.
// Oozing gets harder as the legs come in, so learning to walk pays off.

const growPeriod = 120 * 50 // ticks per new segment at GrowRate 1 (~50 s)

// Bud resets every limb to a one-segment bud, as on a fresh hatchling.
func (p *Plip) Bud() {
	for i := range p.limbs {
		p.limbs[i].grown = 1
	}
}

// LimbGrowth is how many segments each limb has grown, for saving.
func (p *Plip) LimbGrowth() []int {
	out := make([]int, len(p.limbs))
	for i, l := range p.limbs {
		out[i] = l.grown
	}
	return out
}

// SetLimbGrowth restores saved growth. Call before AddPlip.
func (p *Plip) SetLimbGrowth(g []int) {
	for i := range p.limbs {
		if i < len(g) {
			p.limbs[i].grown = max(1, min(g[i], p.limbs[i].segs()))
		}
	}
}

// Legs reports how many legs it has and how grown they are (0..1).
func (p *Plip) Legs() (int, float32) { return p.nLegs, p.legGrowth() }

func (p *Plip) legGrowth() float32 {
	var g, t int
	for _, l := range p.limbs {
		if l.leg >= 0 {
			g += l.grown
			t += l.segs()
		}
	}
	if t == 0 {
		return 0
	}
	return float32(g) / float32(t)
}

// WalkWords says how well it's getting about on its legs.
func (p *Plip) WalkWords() string {
	if p.Motor == nil {
		return ""
	}
	if p.legGrowth() < 0.35 {
		return "legs budding"
	}
	r := p.Motor.Skill / (0.2 * p.G.Speed)
	switch {
	case p.Motor.Tries < 8 || r < 0.15:
		return "flailing its legs"
	case r < 0.45:
		return "first wobbly steps"
	case r < 0.8:
		return "toddling"
	case r < 1.3:
		return "walking"
	}
	return "trotting about"
}

func (p *Plip) grow(w *World) {
	if p.Brain == nil {
		return // gallery plips show what they'll grow into
	}
	rate := p.G.GrowRate
	if rate <= 0 {
		rate = 1
	}
	p.growT += rate
	if p.growT < growPeriod {
		return
	}
	p.growT = 0
	for li := range p.limbs {
		l := &p.limbs[li]
		if l.grown >= l.segs() || p.Mass <= MinMass+6 {
			continue
		}
		if p.sprout(w, li) {
			l.grown++
			p.Drives[Hunger] = clamp01(p.Drives[Hunger] + 0.015)
		}
	}
}

// sprout turns the body drop nearest the limb's end into its next segment.
func (p *Plip) sprout(w *World, li int) bool {
	ex, ey := p.limbAnchor(li)
	if idx := p.limbIdx[li]; p.limbs[li].grown-1 < len(idx) {
		if i := idx[p.limbs[li].grown-1]; i >= 0 {
			ex, ey = w.X[i], w.Y[i]
		}
	}
	best, bd := -1, float32(1e9)
	for i := 0; i < w.N; i++ {
		if int(w.Own[i]) != p.ID || w.Role[i] != RoleBody {
			continue
		}
		if d := hypot(w.X[i]-ex, w.Y[i]-ey); d < bd {
			best, bd = i, d
		}
	}
	if best < 0 {
		return false
	}
	base := p.G.BaseColor()
	c := [3]float32{base[0] * 0.82, base[1] * 0.82, base[2] * 0.85}
	if p.limbs[li].grown >= p.limbs[li].Len {
		c = [3]float32{base[0]*1.1 + 12, base[1]*1.1 + 12, base[2]*1.1 + 12}
	}
	w.Role[best], w.Lobe[best], w.Seg[best] = RoleLimb, uint8(li), uint8(p.limbs[li].grown)
	w.CR[best], w.CG[best], w.CB[best] = c[0], c[1], c[2]
	return true
}

// hip is where a leg joins the underside of the body.
func (p *Plip) hip(li int) (float32, float32) {
	l := p.limbs[li]
	lb := p.lowLobe()
	r := p.LR[lb]
	off := (l.side*0.5 + l.Angle*0.35) * r * p.ax
	return p.CoreX + p.LX[lb] + off*p.Face, p.CoreY + p.LY[lb] + r*p.ay*0.6
}

// legLen is the thigh and shin length; the foot (tip) doesn't count.
func (l *limbInst) legLen() (float32, float32) {
	n := min(l.grown, l.Len)
	n1 := (n + 1) / 2
	return float32(n1) * 1.15, float32(n-n1) * 1.15
}

// walk moves the legs one tick and returns how far the planted feet pushed
// the body along x.
func (p *Plip) walk(w *World, want bool) float32 {
	if p.nLegs == 0 {
		p.legLift = 0
		return 0
	}
	m := p.Motor
	flip := p.Face != p.lastFace
	p.lastFace = p.Face

	var contact [MaxLegs]float32
	for _, l := range p.limbs {
		if l.leg >= 0 && l.contact {
			contact[l.leg] = 1
		}
	}
	var tg [MaxLegs][2]float32
	if want {
		p.walkPh += m.rate()
		tg = m.targets(p.walkPh, contact, p.legLift/(p.Rad+1))
	}

	var maxReach, stiff float32
	hy := float32(0)
	for li := range p.limbs {
		l := &p.limbs[li]
		if l.leg < 0 {
			continue
		}
		var h, k float32
		switch {
		case want:
			h, k = tg[l.leg][0], tg[l.leg][1]
		case p.Resting:
			h, k = 0.3, 1.5 // folds its legs and sits
		default:
			h, k = 0, 0.3 // just stands
		}
		rate := 0.05 + 0.12*l.Stiff
		l.a1 += clamp(h-l.a1, -rate, rate)
		l.a2 += clamp(k-l.a2, -rate, rate)
		L1, L2 := l.legLen()
		s1, c1 := math.Sincos(float64(l.a1))
		s2, c2 := math.Sincos(float64(l.a1 - l.a2))
		l.reach = (L1*float32(c1) + L2*float32(c2)) * (0.8 + 0.2*l.Stiff) // weak legs buckle a bit
		l.fx = L1*float32(s1) + L2*float32(s2)
		maxReach = max(maxReach, l.reach)
		stiff += l.Stiff
		_, hy = p.hip(li)
	}
	stiff /= float32(p.nLegs)

	// the longest leg on the floor holds the body up; without one it sinks
	c0 := w.Floor - hy - p.lift - p.legLift
	target := clamp(maxReach-c0, 0, p.Rad*1.6)
	if target > p.legLift {
		p.legLift += min(target-p.legLift, 0.15+0.25*stiff)
	} else {
		p.legLift = max(target, p.legLift-0.5)
	}
	clear := c0 + p.legLift

	// a planted foot doesn't slide, so the body moves the other way
	var push float32
	n := 0
	for li := range p.limbs {
		l := &p.limbs[li]
		if l.leg < 0 {
			continue
		}
		l.wasContact = l.contact
		l.contact = p.lift < 0.5 && l.reach >= clear-0.6
		if l.contact && l.wasContact && !flip {
			push -= l.fx - l.pfx
			n++
		}
		l.pfx = l.fx
	}
	if n == 0 {
		return 0
	}
	// a belly dragging on the floor holds it back; standing up pays off
	drag := 0.25 + 0.75*clamp(p.legLift/(p.Rad*0.5+1), 0, 1)
	return push / float32(n) * p.Face * drag
}

// placeLeg lays a leg's particles along its thigh, shin and foot.
func (p *Plip) placeLeg(w *World, li int) {
	l := p.limbs[li]
	hx, hy := p.hip(li)
	n := min(l.grown, l.Len)
	n1 := (n + 1) / 2
	s1, c1 := math.Sincos(float64(l.a1))
	s2, c2 := math.Sincos(float64(l.a1 - l.a2))
	tx, ty := float32(s1)*p.Face, float32(c1)
	sx, sy := float32(s2)*p.Face, float32(c2)
	L1 := float32(n1) * 1.15
	kx, ky := hx+tx*L1, hy+ty*L1
	fx, fy := kx+sx*float32(n-n1)*1.15, ky+sy*float32(n-n1)*1.15
	for s, i := range p.limbIdx[li] {
		if i < 0 {
			continue
		}
		var x, y float32
		switch {
		case s < n1:
			d := float32(s+1) * 1.15
			x, y = hx+tx*d, hy+ty*d
		case s < n:
			d := float32(s-n1+1) * 1.15
			x, y = kx+sx*d, ky+sy*d
		default: // toes point forward
			d := float32(s-n+1) * 0.6
			x, y = fx+p.Face*d, fy
		}
		if y > w.Floor {
			y = w.Floor
		}
		w.X[i], w.Y[i] = x, y
	}
}

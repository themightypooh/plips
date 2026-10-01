package sim

import "math"

// Limbs grow in as a plip ages, one segment at a time, each made from a drop
// of its own body (so a well-fed plip grows faster). Legs sprout from the
// top and arch over, the shin drooping down past the body. The motor net
// (motor.go) swings thigh and shin, a foot on the floor stays put, and the
// body is carried along by whatever the planted feet do. The legs can only
// lift as much as they've the strength for, and strength builds slowly with
// use. Oozing gets harder as the legs come in, so learning to walk pays off.

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
	if p.Motor.Strength < 0.3 {
		return "legs too weak to stand"
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

// hip is where a leg sprouts: high on the body, front and back.
func (p *Plip) hip(li int) (float32, float32) {
	l := p.limbs[li]
	top := 0
	for i := 1; i < p.lobes(); i++ {
		if p.LY[i]-p.LR[i]*p.ay < p.LY[top]-p.LR[top]*p.ay {
			top = i
		}
	}
	r := p.LR[top]
	off := (l.side*0.45 + l.Angle*0.3) * r * p.ax
	return p.CoreX + p.LX[top] + off*p.Face, p.CoreY + p.LY[top] - r*p.ay*0.7
}

// legSeg is the spacing of a leg's particles; legs scale with the body so a
// grown one can arch over it and reach the floor.
func (p *Plip) legSeg(l *limbInst) float32 {
	return clamp(p.Rad*p.ay*5/float32(max(l.Len, 1)), 0.9, 1.6)
}

// legSplit is how many of the grown segments form the thigh (which grows
// first and sticks up) and how many the shin (which droops down).
func legSplit(l *limbInst) (int, int) {
	n := min(l.grown, l.Len)
	n1 := min(n, (l.Len*25+99)/100)
	return n1, n - n1
}

// strengthTicks is roughly how long it has to work its legs before they can
// hold it fully up.
const strengthTicks = 120 * 60 * 20

// LegLift is how far its legs are holding its body off the floor, in px.
func (p *Plip) LegLift() float32 { return p.legLift }

// Strength is how much of its weight the legs can hold up (0..1).
func (p *Plip) Strength() float32 {
	if p.Motor == nil {
		return 0
	}
	return p.Motor.Strength
}

// walk moves the legs one tick and returns how far the planted feet pushed
// the body along x. Thighs arch up from the top of the body; shins hang down
// past it to the floor. Angles: a1 tilts the thigh from straight up, a2
// swings the shin from straight down; positive is forward for both.
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
		splay := l.side*0.5 + l.Angle*0.4
		var h, k float32
		switch {
		case want:
			h, k = splay+tg[l.leg][0], l.side*0.15+tg[l.leg][1]
		case p.Resting:
			h, k = splay*1.8, l.side*0.7 // sprawls
		default:
			h, k = splay, l.side*0.2
		}
		rate := 0.05 + 0.12*l.Stiff
		l.a1 += clamp(h-l.a1, -rate, rate)
		l.a2 += clamp(k-l.a2, -rate, rate)
		n1, n2 := legSplit(l)
		sg := p.legSeg(l)
		l.reach = float32(n2)*sg*float32(math.Cos(float64(l.a2))) - float32(n1)*sg*float32(math.Cos(float64(l.a1)))
		maxReach = max(maxReach, l.reach)
		stiff += l.Stiff
		_, hy = p.hip(li)
	}
	stiff /= float32(p.nLegs)

	// legs only hold up as much as they've the strength for
	c0 := w.Floor - hy - p.lift - p.legLift
	target := clamp(maxReach-c0, 0, p.Rad*1.8*m.Strength)
	if target > p.legLift {
		p.legLift += min(target-p.legLift, (0.05+0.25*stiff)*(0.2+m.Strength))
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
		n1, n2 := legSplit(l)
		sg := p.legSeg(l)
		L1, L2 := float32(n1)*sg, float32(n2)*sg
		// a foot that would go through the floor splays out instead
		l.a2e = l.a2
		need := clear + L1*float32(math.Cos(float64(l.a1)))
		if L2 > 0 && L2*float32(math.Cos(float64(l.a2))) > need {
			a := float32(math.Acos(float64(clamp(need/L2, -1, 1))))
			if l.a2 < 0 || (l.a2 == 0 && l.side < 0) {
				a = -a
			}
			l.a2e = a
		}
		l.fx = L1*float32(math.Sin(float64(l.a1))) + L2*float32(math.Sin(float64(l.a2e)))
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
	if want { // working the legs against the floor builds them up
		g := p.legGrowth()
		m.Strength = min(g, m.Strength+(0.5+stiff)/strengthTicks)
	}
	// a belly dragging on the floor holds it back; standing up pays off
	drag := 0.25 + 0.75*clamp(p.legLift/(p.Rad*0.5+1), 0, 1)
	// and weak legs can't push much
	v := push / float32(n) * drag * (0.2 + 0.8*m.Strength)
	return clamp(v, -0.6, 0.6) * p.Face
}

// placeLeg lays a leg's particles up along the thigh, down the shin, then
// the foot.
func (p *Plip) placeLeg(w *World, li int) {
	l := &p.limbs[li]
	hx, hy := p.hip(li)
	n1, n2 := legSplit(l)
	n := n1 + n2
	sg := p.legSeg(l)
	s1, c1 := math.Sincos(float64(l.a1))
	s2, c2 := math.Sincos(float64(l.a2e))
	tx, ty := float32(s1)*p.Face, -float32(c1)
	sx, sy := float32(s2)*p.Face, float32(c2)
	kx, ky := hx+tx*float32(n1)*sg, hy+ty*float32(n1)*sg
	fx, fy := kx+sx*float32(n2)*sg, ky+sy*float32(n2)*sg
	for s, i := range p.limbIdx[li] {
		if i < 0 {
			continue
		}
		var x, y float32
		switch {
		case s < n1:
			d := float32(s+1) * sg
			x, y = hx+tx*d, hy+ty*d
		case s < n:
			d := float32(s-n1+1) * sg
			x, y = kx+sx*d, ky+sy*d
		default: // toes point outward
			d := float32(s-n+1) * 0.6
			side := l.side
			if side == 0 {
				side = 1
			}
			x, y = fx+p.Face*side*d, fy
		}
		if y > w.Floor {
			y = w.Floor
		}
		w.X[i], w.Y[i] = x, y
	}
}

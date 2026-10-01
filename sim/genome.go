package sim

import (
	"fmt"
	"math"
	"math/rand"
)

// Lobe layouts for multi-part bodies.
const (
	LayoutSingle = iota
	LayoutChain  // segments side by side (peanut, caterpillar)
	LayoutStack  // segments on top of each other (snowman)
	LayoutClump  // a triangle cluster
	numLayouts
)

var layoutNames = []string{"single", "chain", "stack", "clump"}

// Limb kinds: thin wobbly strands of liquid hanging off the body.
const (
	LimbFeeler  = iota // sticks up from the head
	LimbTail           // trails behind
	LimbWhisker        // pokes forward/sideways from the face
	LimbNub            // short stub low on the side
	numLimbKinds
)

var limbNames = []string{"feelers", "tail", "whiskers", "nubs"}

type Limb struct {
	Kind   int
	Len    int     // segments
	Angle  float32 // tilt off the kind's base direction, radians
	Stiff  float32 // 0 limp .. 1 stiff
	Tip    bool    // a little blob on the end
	Mirror bool    // comes as a left/right pair
}

// Genome is everything a plip inherits. Body genes shape the look and feel
// of the liquid; mind genes shape how the brain learns.
type Genome struct {
	// body
	Mass      int     // starting particle count
	Layout    int     // lobe arrangement
	Lobes     int     // 1..4
	LobeGap   float32 // 0.5 merged .. 1.4 distinct segments
	LobeTaper float32 // size falloff along the lobes, 0.6..1
	Aspect    float32 // 0.5 tall .. 2 wide
	Soft      float32 // 0 floppy .. 1 firm
	Taut      float32 // 0 loose skin .. 1 drum-tight
	Sag       float32 // 0 holds its posture .. 1 slumps under gravity
	Pack      float32 // body radius scale, small = pressurised balloon
	Jiggle    float32 // 0 gloopy .. 1 jelly
	Lumpy     float32 // 0 smooth .. 1 knobbly outline
	LumpK     int     // number of lumps around the outline
	Breath    float32 // breathing amplitude
	Nucleus   float32 // fraction of particles forming a visible core
	Limbs     []Limb

	Skin        bool    // a membrane holds the liquid in
	SkinStretch float32 // 0 tight .. 1 stretchy and wobbly
	SkinBend    float32 // 0 crinkly .. 1 smooth and stiff
	SkinWrinkle float32 // fine ripples in the outline
	SkinShade   float32 // 0 dark rim .. 1 pale glossy rim
	SkinClear   float32 // 0 milky .. 1 see-through

	Hue, Sat, Val float32
	Speckle       float32 // per-particle colour noise
	Spots         float32 // fraction of particles in the second colour
	Hue2          float32
	Rim           float32 // outline darkness 0..1
	Alpha         float32 // opacity 0.55..1

	Eyes    int // 0..3 dark dot eyes
	EyeSize int // 1..2 px
	EyeGap  float32
	EyeHigh float32

	Hoppy float32
	Speed float32

	// mind
	LR       float32       // learning rate
	Temp     float32       // decision randomness
	Curious  float32       // how rewarding surprise feels
	Taste    [NCol]float32 // how filling each colour is to this one
	Metab    float32
	Instinct int64 // seed for innate brain wiring; also the genome's identity
}

func lerp(a, b, t float32) float32 { return a + (b-a)*t }

func randomLimb(r *rand.Rand) Limb {
	l := Limb{Kind: r.Intn(numLimbKinds), Stiff: r.Float32(), Tip: r.Float32() < 0.35}
	l.Angle = (r.Float32()*2 - 1) * 0.6
	switch l.Kind {
	case LimbFeeler:
		l.Len, l.Mirror = 3+r.Intn(6), r.Float32() < 0.7
	case LimbTail:
		l.Len = 3 + r.Intn(8)
	case LimbWhisker:
		l.Len, l.Mirror = 2+r.Intn(4), r.Float32() < 0.5
	case LimbNub:
		l.Len, l.Mirror, l.Tip = 1+r.Intn(3), true, false
	}
	return l
}

// RandomGenome rolls a completely fresh plip.
func RandomGenome(r *rand.Rand) Genome {
	f := r.Float32
	g := Genome{
		Mass:      55 + r.Intn(100),
		Layout:    LayoutSingle,
		Lobes:     1,
		LobeGap:   lerp(0.6, 1.3, f()),
		LobeTaper: lerp(0.55, 1, f()),
		Aspect:    lerp(0.5, 2, f()*f()+f()*0.3),
		Soft:      f(),
		Taut:      f(),
		Sag:       f(),
		Pack:      lerp(0.75, 1.15, f()),
		Jiggle:    f(),
		LumpK:     3 + r.Intn(5),
		Breath:    f() * f(),
		Hue:       f(),
		Sat:       lerp(0.12, 0.5, f()),
		Val:       lerp(0.4, 0.88, f()),
		Speckle:   f() * 26,
		Hue2:      f(),
		Rim:       lerp(0.2, 0.9, f()),
		Alpha:     lerp(0.6, 1, f()),
		Eyes:      1 + r.Intn(3),
		EyeSize:   1,
		EyeGap:    lerp(0.15, 0.55, f()),
		EyeHigh:   lerp(0.1, 0.6, f()),
		Hoppy:     f(),
		Speed:     lerp(0.6, 1.4, f()),
		LR:        lerp(0.02, 0.08, f()),
		Temp:      lerp(0.1, 0.35, f()),
		Curious:   lerp(0.5, 1.5, f()),
		Metab:     lerp(0.7, 1.3, f()),
		Instinct:  r.Int63(),
	}
	if f() < 0.55 {
		g.Layout = 1 + r.Intn(numLayouts-1)
		g.Lobes = 2 + r.Intn(2)
		if g.Layout == LayoutChain && f() < 0.4 {
			g.Lobes = 4
		}
	}
	if f() < 0.4 {
		g.Lumpy = lerp(0.2, 1, f())
	}
	g.Skin = f() < 0.9
	g.SkinStretch, g.SkinBend, g.SkinShade, g.SkinClear = f(), f(), f(), f()
	if f() < 0.3 {
		g.SkinWrinkle = f()
	}
	if f() < 0.3 {
		g.Nucleus = lerp(0.06, 0.25, f())
	}
	if f() < 0.35 {
		g.Spots = lerp(0.05, 0.35, f())
	}
	if f() < 0.2 {
		g.EyeSize = 2
	}
	if f() < 0.08 {
		g.Eyes = 0
	}
	for n := r.Intn(4); n > 0 && f() < 0.8; n-- {
		g.Limbs = append(g.Limbs, randomLimb(r))
	}
	for i := range g.Taste {
		g.Taste[i] = lerp(0.5, 1.5, f())
	}
	return g
}

// Mutate returns a close relative of g. amount 0..1 sets how far it drifts.
func (g Genome) Mutate(r *rand.Rand, amount float32) Genome {
	n := func(v, lo, hi float32) float32 {
		v += (r.Float32()*2 - 1) * (hi - lo) * 0.25 * amount
		return float32(math.Max(float64(lo), math.Min(float64(hi), float64(v))))
	}
	wrap := func(v float32) float32 {
		v += (r.Float32()*2 - 1) * 0.12 * amount
		return v - float32(math.Floor(float64(v)))
	}
	chance := func(p float32) bool { return r.Float32() < p*amount }
	m := g
	m.Mass = int(n(float32(g.Mass), 40, 180))
	m.LobeGap = n(g.LobeGap, 0.5, 1.4)
	m.LobeTaper = n(g.LobeTaper, 0.5, 1)
	m.Aspect = n(g.Aspect, 0.45, 2.1)
	m.Soft = n(g.Soft, 0, 1)
	m.Taut = n(g.Taut, 0, 1)
	m.Sag = n(g.Sag, 0, 1)
	m.Pack = n(g.Pack, 0.7, 1.2)
	m.Jiggle = n(g.Jiggle, 0, 1)
	m.Breath = n(g.Breath, 0, 1)
	m.Hue = wrap(g.Hue)
	m.Sat = n(g.Sat, 0.05, 0.6)
	m.Val = n(g.Val, 0.3, 0.92)
	m.Speckle = n(g.Speckle, 0, 30)
	m.Hue2 = wrap(g.Hue2)
	m.Rim = n(g.Rim, 0, 1)
	m.Alpha = n(g.Alpha, 0.5, 1)
	m.EyeGap = n(g.EyeGap, 0.1, 0.6)
	m.EyeHigh = n(g.EyeHigh, 0.05, 0.65)
	m.Hoppy = n(g.Hoppy, 0, 1)
	m.Speed = n(g.Speed, 0.5, 1.5)
	m.LR = n(g.LR, 0.01, 0.1)
	m.Temp = n(g.Temp, 0.06, 0.45)
	m.Curious = n(g.Curious, 0.2, 2)
	m.Metab = n(g.Metab, 0.6, 1.4)
	m.SkinStretch = n(g.SkinStretch, 0, 1)
	m.SkinBend = n(g.SkinBend, 0, 1)
	m.SkinShade = n(g.SkinShade, 0, 1)
	m.SkinClear = n(g.SkinClear, 0, 1)
	if g.SkinWrinkle > 0 || chance(0.15) {
		m.SkinWrinkle = n(g.SkinWrinkle, 0, 1)
	}
	if chance(0.08) {
		m.Skin = !g.Skin
	}
	if g.Lumpy > 0 || chance(0.15) {
		m.Lumpy = n(g.Lumpy, 0, 1)
	}
	if chance(0.15) {
		m.LumpK = 3 + r.Intn(5)
	}
	if g.Nucleus > 0 || chance(0.1) {
		m.Nucleus = n(g.Nucleus, 0, 0.3)
	}
	if g.Spots > 0 || chance(0.1) {
		m.Spots = n(g.Spots, 0, 0.4)
	}
	for i := range m.Taste {
		m.Taste[i] = n(g.Taste[i], 0.3, 1.7)
	}
	// limbs: tweak, lose or grow one
	m.Limbs = append([]Limb(nil), g.Limbs...)
	for i := range m.Limbs {
		l := &m.Limbs[i]
		l.Stiff = n(l.Stiff, 0, 1)
		l.Angle = n(l.Angle, -0.8, 0.8)
		if chance(0.3) {
			l.Len = int(n(float32(l.Len), 1, 11))
		}
		if chance(0.1) {
			l.Tip = !l.Tip
		}
	}
	if len(m.Limbs) > 0 && chance(0.15) {
		k := r.Intn(len(m.Limbs))
		m.Limbs = append(m.Limbs[:k], m.Limbs[k+1:]...)
	}
	if len(m.Limbs) < 4 && chance(0.2) {
		m.Limbs = append(m.Limbs, randomLimb(r))
	}
	// rarer, chunkier changes
	if chance(0.15) {
		m.Eyes = r.Intn(4)
	}
	if chance(0.15) {
		m.EyeSize = 1 + r.Intn(2)
	}
	if chance(0.12) {
		m.Layout = r.Intn(numLayouts)
		m.Lobes = 1
		if m.Layout != LayoutSingle {
			m.Lobes = 2 + r.Intn(2)
			if m.Layout == LayoutChain && r.Intn(3) == 0 {
				m.Lobes = 4
			}
		}
	}
	m.Instinct = r.Int63()
	return m
}

// BaseColor is the body colour as RGB 0..255.
func (g Genome) BaseColor() [3]float32 { return hsv(g.Hue, g.Sat, g.Val) }

// SpotColor is the second body colour.
func (g Genome) SpotColor() [3]float32 {
	return hsv(g.Hue2, clamp01(g.Sat+0.1), clamp01(g.Val*0.8))
}

// Describe is a short readable summary of the body genes.
func (g Genome) Describe() string {
	shape := layoutNames[g.Layout]
	if g.Lobes > 1 {
		shape = fmt.Sprintf("%d-%s", g.Lobes, shape)
	}
	if g.Lobes == 1 {
		shape = word(g.Aspect-0.5, "tall", "round", "wide")
	}
	words := []string{shape, word(g.Soft, "floppy", "", "firm"), word(g.Sag, "", "", "slumpy")}
	if g.Lumpy > 0.4 {
		words = append(words, "lumpy")
	}
	if !g.Skin {
		words = append(words, "skinless")
	} else if g.SkinStretch > 0.7 {
		words = append(words, "stretchy")
	}
	seen := map[int]bool{}
	for _, l := range g.Limbs {
		if !seen[l.Kind] {
			words = append(words, limbNames[l.Kind])
			seen[l.Kind] = true
		}
	}
	s := ""
	for _, w := range words {
		if w != "" {
			if s != "" {
				s += " "
			}
			s += w
		}
	}
	return s
}

func word(v float32, lo, mid, hi string) string {
	switch {
	case v < 0.33:
		return lo
	case v > 0.66:
		return hi
	}
	return mid
}

func hsv(h, s, v float32) [3]float32 {
	h = (h - float32(math.Floor(float64(h)))) * 6
	i := int(h)
	f := h - float32(i)
	p, q, t := v*(1-s), v*(1-s*f), v*(1-s*(1-f))
	var r, g, b float32
	switch i % 6 {
	case 0:
		r, g, b = v, t, p
	case 1:
		r, g, b = q, v, p
	case 2:
		r, g, b = p, v, t
	case 3:
		r, g, b = p, q, v
	case 4:
		r, g, b = t, p, v
	default:
		r, g, b = v, p, q
	}
	return [3]float32{r * 255, g * 255, b * 255}
}

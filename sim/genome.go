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

// Genome is everything a critter inherits. Body genes shape the look and
// feel of the liquid; mind genes shape how the brain learns.
type Genome struct {
	// body
	Mass     int     // starting particle count
	Layout   int     // lobe arrangement
	Lobes    int     // 1..4
	LobeGap  float32 // 0.5 merged .. 1.1 distinct segments
	LobeTaper float32 // size falloff along the lobes, 0.6..1
	Aspect   float32 // 0.6 tall .. 1.6 wide
	Soft     float32 // 0 floppy puddle .. 1 firm
	Taut     float32 // 0 loose skin .. 1 drum-tight
	Pack     float32 // body radius scale, small = pressurised balloon
	Jiggle   float32 // 0 gloopy .. 1 jelly
	Breath   float32 // breathing amplitude
	Nucleus  float32 // fraction of particles forming a visible core
	Hue, Sat, Val float32
	Speckle  float32 // per-particle colour noise
	Spots    float32 // fraction of particles in the second colour
	Hue2     float32
	Rim      float32 // outline darkness 0..1
	Alpha    float32 // opacity 0.55..1
	Eyes     int     // 0..3
	EyeSize  int     // 1..3 px
	EyeGap   float32
	EyeHigh  float32
	Pupil    int // 0 dot, 1 none, 2 tall slit
	Hoppy    float32
	Speed    float32

	// mind
	LR    float32    // learning rate
	Temp  float32    // decision randomness (curiosity)
	Taste [NCol]float32 // how filling each colour is to this one
	Metab float32
	Instinct int64   // seed for innate brain wiring
}

func lerp(a, b, t float32) float32 { return a + (b-a)*t }

// RandomGenome rolls a completely fresh critter.
func RandomGenome(r *rand.Rand) Genome {
	f := r.Float32
	g := Genome{
		Mass:      55 + r.Intn(90),
		Layout:    LayoutSingle,
		Lobes:     1,
		LobeGap:   lerp(0.5, 1.05, f()),
		LobeTaper: lerp(0.6, 1, f()),
		Aspect:    lerp(0.65, 1.6, f()),
		Soft:      f(),
		Taut:      f(),
		Pack:      lerp(0.75, 1.15, f()),
		Jiggle:    f(),
		Breath:    f() * f(),
		Hue:       f(),
		Sat:       lerp(0.12, 0.5, f()),
		Val:       lerp(0.4, 0.88, f()),
		Speckle:   f() * 26,
		Hue2:      f(),
		Rim:       lerp(0.2, 0.9, f()),
		Alpha:     lerp(0.55, 1, f()),
		Eyes:      1 + r.Intn(3),
		EyeSize:   1 + r.Intn(3),
		EyeGap:    lerp(0.15, 0.5, f()),
		EyeHigh:   lerp(0.1, 0.6, f()),
		Pupil:     r.Intn(3),
		Hoppy:     f(),
		Speed:     lerp(0.6, 1.4, f()),
		LR:        lerp(0.2, 0.7, f()),
		Temp:      lerp(0.12, 0.4, f()),
		Metab:     lerp(0.7, 1.3, f()),
		Instinct:  r.Int63(),
	}
	if f() < 0.45 {
		g.Layout = 1 + r.Intn(numLayouts-1)
		g.Lobes = 2 + r.Intn(2)
		if g.Layout == LayoutChain && f() < 0.4 {
			g.Lobes = 4
		}
	}
	if f() < 0.35 {
		g.Nucleus = lerp(0.06, 0.25, f())
	}
	if f() < 0.35 {
		g.Spots = lerp(0.05, 0.35, f())
	}
	if f() < 0.12 {
		g.Eyes = 0
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
	m := g
	m.Mass = int(n(float32(g.Mass), 40, 170))
	m.LobeGap = n(g.LobeGap, 0.5, 1.1)
	m.LobeTaper = n(g.LobeTaper, 0.6, 1)
	m.Aspect = n(g.Aspect, 0.6, 1.7)
	m.Soft = n(g.Soft, 0, 1)
	m.Taut = n(g.Taut, 0, 1)
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
	m.EyeGap = n(g.EyeGap, 0.1, 0.55)
	m.EyeHigh = n(g.EyeHigh, 0.05, 0.65)
	m.Hoppy = n(g.Hoppy, 0, 1)
	m.Speed = n(g.Speed, 0.5, 1.5)
	m.LR = n(g.LR, 0.1, 0.8)
	m.Temp = n(g.Temp, 0.08, 0.5)
	m.Metab = n(g.Metab, 0.6, 1.4)
	if g.Nucleus > 0 || r.Float32() < 0.1*amount {
		m.Nucleus = n(g.Nucleus, 0, 0.3)
	}
	if g.Spots > 0 || r.Float32() < 0.1*amount {
		m.Spots = n(g.Spots, 0, 0.4)
	}
	for i := range m.Taste {
		m.Taste[i] = n(g.Taste[i], 0.3, 1.7)
	}
	// rarer, chunkier changes
	if r.Float32() < 0.15*amount {
		m.Eyes = r.Intn(4)
	}
	if r.Float32() < 0.15*amount {
		m.EyeSize = 1 + r.Intn(3)
	}
	if r.Float32() < 0.1*amount {
		m.Pupil = r.Intn(3)
	}
	if r.Float32() < 0.12*amount {
		m.Layout = r.Intn(numLayouts)
		m.Lobes = 1
		if m.Layout != LayoutSingle {
			m.Lobes = 2 + r.Intn(3)
			if m.Layout != LayoutChain && m.Lobes > 3 {
				m.Lobes = 3
			}
		}
	}
	m.Instinct = r.Int63()
	return m
}

// Cross mixes two parents gene by gene.
func Cross(a, b Genome, r *rand.Rand) Genome {
	pick := func(x, y float32) float32 {
		if r.Intn(2) == 0 {
			return x
		}
		return y
	}
	c := a
	if r.Intn(2) == 0 {
		c.Layout, c.Lobes = b.Layout, b.Lobes
	}
	if r.Intn(2) == 0 {
		c.Eyes, c.EyeSize, c.Pupil = b.Eyes, b.EyeSize, b.Pupil
	}
	c.Mass = (a.Mass + b.Mass) / 2
	c.LobeGap, c.LobeTaper, c.Aspect = pick(a.LobeGap, b.LobeGap), pick(a.LobeTaper, b.LobeTaper), pick(a.Aspect, b.Aspect)
	c.Soft, c.Taut, c.Pack, c.Jiggle = pick(a.Soft, b.Soft), pick(a.Taut, b.Taut), pick(a.Pack, b.Pack), pick(a.Jiggle, b.Jiggle)
	c.Breath, c.Nucleus = pick(a.Breath, b.Breath), pick(a.Nucleus, b.Nucleus)
	c.Hue, c.Sat, c.Val = pick(a.Hue, b.Hue), (a.Sat+b.Sat)/2, (a.Val+b.Val)/2
	c.Speckle, c.Spots, c.Hue2 = pick(a.Speckle, b.Speckle), pick(a.Spots, b.Spots), pick(a.Hue2, b.Hue2)
	c.Rim, c.Alpha = pick(a.Rim, b.Rim), pick(a.Alpha, b.Alpha)
	c.EyeGap, c.EyeHigh = pick(a.EyeGap, b.EyeGap), pick(a.EyeHigh, b.EyeHigh)
	c.Hoppy, c.Speed = pick(a.Hoppy, b.Hoppy), pick(a.Speed, b.Speed)
	c.LR, c.Temp, c.Metab = pick(a.LR, b.LR), pick(a.Temp, b.Temp), pick(a.Metab, b.Metab)
	for i := range c.Taste {
		c.Taste[i] = pick(a.Taste[i], b.Taste[i])
	}
	return c.Mutate(r, 0.3)
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
	feel := word(g.Soft, "floppy", "soft", "firm")
	skin := word(g.Taut, "loose", "", "taut")
	j := word(g.Jiggle, "gloopy", "", "jiggly")
	s := fmt.Sprintf("%s %s", shape, feel)
	for _, w := range []string{skin, j} {
		if w != "" {
			s += " " + w
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

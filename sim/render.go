package sim

import "math"

// Renderer turns particles into pixels. Output is RGBA, premultiplied alpha
// (what Ebitengine's WritePixels expects).
type Renderer struct {
	W, H                             int
	den, ar, ag, ab, aa, rim, sh, lb []float32
	skin                             []uint8 // plip ID whose membrane covers the pixel
	// Alpha of the pixel last drawn; used for click-through hit tests.
	Alpha []uint8
}

func NewRenderer(w, h int) *Renderer {
	n := w * h
	f := func() []float32 { return make([]float32, n) }
	return &Renderer{W: w, H: h, den: f(), ar: f(), ag: f(), ab: f(), aa: f(), rim: f(), sh: f(), lb: f(), Alpha: make([]uint8, n)}
}

const thresh = 0.55

// Render draws the world into dst (W*H*4 bytes). bg is an opaque RGBA
// background of the same size, or nil for a transparent one.
func (r *Renderer) Render(w *World, dst, bg []byte) {
	W, H := r.W, r.H
	for i := range r.den {
		r.den[i], r.ar[i], r.ag[i], r.ab[i], r.aa[i], r.rim[i], r.sh[i], r.lb[i] = 0, 0, 0, 0, 0, 0, 0, 0
	}
	for i := 0; i < w.N; i++ {
		alpha, rim, lobe := float32(0.93), float32(0.55), float32(0)
		if o := w.Own[i]; o != 0 {
			g := w.Plips[o-1].G
			alpha, rim = g.Alpha, g.Rim
			if w.Role[i] == RoleBody {
				lobe = float32(w.Lobe[i])
			}
		}
		x, y := w.X[i], w.Y[i]
		bx, by := int(x), int(y)
		for oy := -1; oy <= 1; oy++ {
			py := by + oy
			if py < 0 || py >= H {
				continue
			}
			for ox := -1; ox <= 1; ox++ {
				px := bx + ox
				if px < 0 || px >= W {
					continue
				}
				dx, dy := float32(px)+0.5-x, float32(py)+0.5-y
				d2 := dx*dx + dy*dy
				if d2 < 2.1 {
					wt := 1 - d2/2.1
					o := py*W + px
					r.den[o] += wt
					r.ar[o] += wt * w.CR[i]
					r.ag[o] += wt * w.CG[i]
					r.ab[o] += wt * w.CB[i]
					r.aa[o] += wt * alpha
					r.rim[o] += wt * rim
					r.lb[o] += wt * lobe
				}
			}
		}
	}

	// floor shadows under plips
	for _, p := range w.Plips {
		if p.Mass < 10 {
			continue
		}
		fy := int(w.Floor) + 1
		k := 0.45 - clamp(p.lift/(p.Rad+1), 0, 1)*0.3
		for x := int(p.MX - p.Rad*p.ax); x <= int(p.MX+p.Rad*p.ax); x++ {
			if x >= 0 && x < W && fy < H {
				r.sh[fy*W+x] = k
				if fy+1 < H {
					r.sh[(fy+1)*W+x] = k * 0.5
				}
			}
		}
	}

	r.skinMask(w)
	full := func(o int) bool { return r.den[o] > thresh || r.skin[o] != 0 }
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			o := y*W + x
			pp := o * 4
			if d := r.den[o]; d > thresh {
				up := y > 0 && full(o-W)
				dn := y < H-1 && full(o+W)
				lf := x > 0 && full(o-1)
				rt := x < W-1 && full(o+1)
				s := 0.92 + float32(math.Min(float64(d), 3))*0.04
				if !up {
					s = 1.28
				} else if !dn || !lf || !rt {
					s = 1 - 0.55*(r.rim[o]/d)
				} else if y > 1 && !full(o-2*W) {
					s = 1.1
				} else if seam(r.lb[o]/d, r.lb[o+1]/r.den[o+1]) || seam(r.lb[o]/d, r.lb[o+W]/r.den[o+W]) {
					s = 1 - 0.3*(r.rim[o]/d) // crease where two body lobes meet
				}
				inv := s / d
				cr, cg, cb := q8(r.ar[o]*inv), q8(r.ag[o]*inv), q8(r.ab[o]*inv)
				a := clamp(r.aa[o]/d, 0, 1)
				if bg != nil {
					dst[pp] = byte(float32(cr)*a + float32(bg[pp])*(1-a))
					dst[pp+1] = byte(float32(cg)*a + float32(bg[pp+1])*(1-a))
					dst[pp+2] = byte(float32(cb)*a + float32(bg[pp+2])*(1-a))
					dst[pp+3] = 255
				} else {
					dst[pp] = byte(float32(cr) * a)
					dst[pp+1] = byte(float32(cg) * a)
					dst[pp+2] = byte(float32(cb) * a)
					dst[pp+3] = byte(a * 255)
				}
			} else if id := r.skin[o]; id != 0 {
				r.skinFill(w.Plips[id-1], dst, bg, pp)
			} else if bg != nil {
				k := 1 - r.sh[o]
				dst[pp] = byte(float32(bg[pp]) * k)
				dst[pp+1] = byte(float32(bg[pp+1]) * k)
				dst[pp+2] = byte(float32(bg[pp+2]) * k)
				dst[pp+3] = 255
			} else {
				a := r.sh[o] * 0.6
				dst[pp], dst[pp+1], dst[pp+2], dst[pp+3] = 0, 0, 0, byte(a*255)
			}
			r.Alpha[o] = dst[pp+3]
		}
	}
	for _, p := range w.Plips {
		r.skinEdge(w, p, dst)
	}
	for _, p := range w.Plips {
		r.eyes(p, dst)
	}
}

// skinMask marks the pixels inside each membrane.
func (r *Renderer) skinMask(w *World) {
	if len(r.skin) != r.W*r.H {
		r.skin = make([]uint8, r.W*r.H)
	}
	for i := range r.skin {
		r.skin[i] = 0
	}
	for _, p := range w.Plips {
		xs, ys := p.skinPoly(w)
		if xs == nil {
			continue
		}
		x0, y0, x1, y1 := xs[0], ys[0], xs[0], ys[0]
		for k := range xs {
			x0, x1 = min(x0, xs[k]), max(x1, xs[k])
			y0, y1 = min(y0, ys[k]), max(y1, ys[k])
		}
		for y := max(0, int(y0)); y <= min(r.H-1, int(y1)+1); y++ {
			for x := max(0, int(x0)); x <= min(r.W-1, int(x1)+1); x++ {
				if pointIn(xs, ys, float32(x)+0.5, float32(y)+0.5) {
					r.skin[y*r.W+x] = uint8(p.ID)
				}
			}
		}
	}
}

// skinFill paints membrane where no liquid is showing: the skin colour,
// see-through to whatever is behind by the SkinClear gene.
func (r *Renderer) skinFill(p *Plip, dst, bg []byte, pp int) {
	c := p.G.BaseColor()
	a := lerp(0.92, 0.35, p.G.SkinClear) * p.G.Alpha
	cr, cg, cb := q8(c[0]*0.86), q8(c[1]*0.86), q8(c[2]*0.88)
	if bg != nil {
		dst[pp] = byte(float32(cr)*a + float32(bg[pp])*(1-a))
		dst[pp+1] = byte(float32(cg)*a + float32(bg[pp+1])*(1-a))
		dst[pp+2] = byte(float32(cb)*a + float32(bg[pp+2])*(1-a))
		dst[pp+3] = 255
	} else {
		dst[pp], dst[pp+1], dst[pp+2], dst[pp+3] = byte(float32(cr)*a), byte(float32(cg)*a), byte(float32(cb)*a), byte(a*255)
	}
}

// skinEdge draws the membrane outline, with a glossy top for pale skins.
func (r *Renderer) skinEdge(w *World, p *Plip, dst []byte) {
	xs, ys := p.skinPoly(w)
	if xs == nil {
		return
	}
	c := p.G.BaseColor()
	k := lerp(0.5, 1.35, p.G.SkinShade)
	rim := [3]uint8{q8(c[0]*k + 6), q8(c[1]*k + 6), q8(c[2]*k + 6)}
	gloss := [3]uint8{q8(c[0]*1.3 + 30), q8(c[1]*1.3 + 30), q8(c[2]*1.3 + 30)}
	n := len(xs)
	for k := 0; k < n; k++ {
		j := (k + 1) % n
		x0, y0 := int(math.Floor(float64(xs[k]))), int(math.Floor(float64(ys[k])))
		x1, y1 := int(math.Floor(float64(xs[j]))), int(math.Floor(float64(ys[j])))
		dx, dy := abs(x1-x0), -abs(y1-y0)
		sx, sy := 1, 1
		if x0 > x1 {
			sx = -1
		}
		if y0 > y1 {
			sy = -1
		}
		e := dx + dy
		for {
			col := rim
			// the top-left of the outline catches the light
			if p.G.SkinShade > 0.45 && ys[k] < p.MY-p.Rad*0.4 && xs[k] < p.MX {
				col = gloss
			}
			r.set(dst, x0, y0, col)
			if x0 == x1 && y0 == y1 {
				break
			}
			e2 := 2 * e
			if e2 >= dy {
				e += dy
				x0 += sx
			}
			if e2 <= dx {
				e += dx
				y0 += sy
			}
		}
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func seam(a, b float32) bool { return a-b > 0.45 || b-a > 0.45 }

func q8(v float32) uint8 {
	v = float32(math.Round(float64(v)/10) * 10)
	return u8(v)
}

func (r *Renderer) set(dst []byte, x, y int, c [3]uint8) {
	if x < 0 || y < 0 || x >= r.W || y >= r.H {
		return
	}
	o := y*r.W + x
	p := o * 4
	dst[p], dst[p+1], dst[p+2], dst[p+3] = c[0], c[1], c[2], 255
	r.Alpha[o] = 255
}

var eyeDark = [3]uint8{20, 20, 24}

// eyes are small dark dots on the head.
func (r *Renderer) eyes(p *Plip, dst []byte) {
	g := p.G
	if g.Eyes == 0 || p.Mass < 12 {
		return
	}
	hr := p.LR[0]
	cx := p.HeadX + p.Face*hr*0.25*p.ax
	cy := p.HeadY - hr*p.ay*g.EyeHigh
	s := g.EyeSize
	if s < 1 {
		s = 1
	}
	gap := float32(math.Max(float64(s)+1, float64(hr*g.EyeGap*2*p.ax)))
	asleep := p.Resting
	if p.Blink > 0 && !asleep {
		return // a blink: the dots vanish for a moment
	}
	for e := 0; e < g.Eyes; e++ {
		off := (float32(e) - float32(g.Eyes-1)/2) * gap
		ex := int(math.Round(float64(cx + off - float32(s-1)/2)))
		ey := int(math.Round(float64(cy)))
		if g.Eyes == 3 && e == 1 {
			ey -= s
		}
		switch {
		case asleep || p.Flinch > 0: // squeezed shut: a short dash
			r.set(dst, ex, ey+s-1, eyeDark)
			r.set(dst, ex+1, ey+s-1, eyeDark)
		case p.Happy > 0: // pleased: dots lift a pixel
			for yy := 0; yy < s; yy++ {
				for xx := 0; xx < s; xx++ {
					r.set(dst, ex+xx, ey+yy-1, eyeDark)
				}
			}
		default:
			for yy := 0; yy < s; yy++ {
				for xx := 0; xx < s; xx++ {
					r.set(dst, ex+xx, ey+yy, eyeDark)
				}
			}
		}
	}
}

// Hit reports whether the pixel at (x, y) is drawn (plip, liquid or eye).
func (r *Renderer) Hit(x, y int) bool {
	if x < 0 || y < 0 || x >= r.W || y >= r.H {
		return false
	}
	return r.Alpha[y*r.W+x] > 100
}

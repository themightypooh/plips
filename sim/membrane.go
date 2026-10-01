package sim

import "math"

// The membrane is a closed ring of skin particles around a plip's liquid.
// Neighbouring nodes are tied together, the ring resists bending, and an
// area constraint inflates it like a water balloon. The liquid inside can
// slosh but can't get out.

// bodyCentre is the middle of the lobes, relative to the core.
func (p *Plip) bodyCentre() (float32, float32) {
	n := p.lobes()
	var x, y float32
	for i := 0; i < n; i++ {
		x += p.LX[i]
		y += p.LY[i]
	}
	return x / float32(n), y / float32(n)
}

// envelope is how far the body's outline reaches from the body centre at
// angle th: the furthest edge of any lobe along that ray, plus lumps.
func (p *Plip) envelope(th float64) float32 {
	cx, cy := p.bodyCentre()
	dx, dy := math.Cos(th), math.Sin(th)
	best := 0.0
	for i := 0; i < p.lobes(); i++ {
		// ray from (cx,cy) against the ellipse centred on the lobe
		a, b := float64(p.LR[i]*p.ax*p.breathScale), float64(p.LR[i]*p.ay*p.breathScale)
		ox, oy := float64(cx-p.LX[i]), float64(cy-p.LY[i])
		A := dx*dx/(a*a) + dy*dy/(b*b)
		B := 2 * (ox*dx/(a*a) + oy*dy/(b*b))
		C := ox*ox/(a*a) + oy*oy/(b*b) - 1
		disc := B*B - 4*A*C
		if disc < 0 {
			continue
		}
		if t := (-B + math.Sqrt(disc)) / (2 * A); t > best {
			best = t
		}
	}
	if best == 0 {
		best = float64(p.Rad)
	}
	r := float32(best)
	if p.G.Lumpy > 0 {
		r *= 1 + p.G.Lumpy*0.22*float32(math.Sin(float64(p.G.LumpK)*th+p.lumpPh))
	}
	if p.G.SkinWrinkle > 0 {
		r *= 1 + p.G.SkinWrinkle*0.06*float32(math.Sin(13*th+p.lumpPh*3))
	}
	return r
}

func (p *Plip) skinNodes() int {
	n := int(2 * math.Pi * float64(p.Rad) * 1.1 / 1.5)
	if n < 16 {
		n = 16
	}
	if n > 120 {
		n = 120
	}
	return n
}

func (p *Plip) spawnSkin(w *World, col [3]float32) {
	n := p.skinNodes()
	p.skinIdx = make([]int32, n)
	bx, by := p.bodyCentre()
	for k := 0; k < n; k++ {
		th := 2 * math.Pi * float64(k) / float64(n)
		r := p.envelope(th)
		w.addRaw(p.CoreX+bx+float32(math.Cos(th))*r, p.CoreY+by+float32(math.Sin(th))*r,
			0, uint8(p.ID), 0, RoleSkin, col, 0, 0)
		w.Seg[w.N-1] = uint8(k)
	}
}

// skinPoly returns the ring's node positions in order (nil if broken).
func (p *Plip) skinPoly(w *World) ([]float32, []float32) {
	n := len(p.skinIdx)
	if n < 3 {
		return nil, nil
	}
	xs, ys := make([]float32, n), make([]float32, n)
	for k, i := range p.skinIdx {
		if i < 0 {
			return nil, nil
		}
		xs[k], ys[k] = w.X[i], w.Y[i]
	}
	return xs, ys
}

func polyArea(xs, ys []float32) float32 {
	var a float32
	n := len(xs)
	for k := 0; k < n; k++ {
		j := (k + 1) % n
		a += xs[k]*ys[j] - xs[j]*ys[k]
	}
	return a / 2
}

func pointIn(xs, ys []float32, x, y float32) bool {
	in := false
	n := len(xs)
	for k, j := 0, n-1; k < n; j, k = k, k+1 {
		if (ys[k] > y) != (ys[j] > y) && x < (xs[j]-xs[k])*(y-ys[k])/(ys[j]-ys[k])+xs[k] {
			in = !in
		}
	}
	return in
}

// solveSkin runs the membrane constraints and keeps the liquid inside.
func (p *Plip) solveSkin(w *World) {
	xs, ys := p.skinPoly(w)
	if xs == nil {
		return
	}
	n := len(xs)
	g := p.G
	bx, by := p.bodyCentre()
	cx, cy := p.CoreX+bx, p.CoreY+by

	// target outline and its area/perimeter
	tx, ty := make([]float32, n), make([]float32, n)
	var perim float32
	for k := 0; k < n; k++ {
		th := 2 * math.Pi * float64(k) / float64(n)
		r := p.envelope(th)
		tx[k], ty[k] = cx+float32(math.Cos(th))*r, cy+float32(math.Sin(th))*r
	}
	for k := 0; k < n; k++ {
		j := (k + 1) % n
		perim += hypot(tx[j]-tx[k], ty[j]-ty[k])
	}
	area0 := polyArea(tx, ty)
	rest := perim / float32(n) * lerp(1.0, 1.12, g.SkinStretch)

	// 1. pull toward the shape the genes want
	kShape := lerp(0.015, 0.14, g.Soft)
	for k := 0; k < n; k++ {
		xs[k] += (tx[k] - xs[k]) * kShape
		ys[k] += (ty[k] - ys[k]) * kShape
	}
	stiff := lerp(0.9, 0.25, g.SkinStretch)
	bend := lerp(0.02, 0.35, g.SkinBend)
	for it := 0; it < 3; it++ {
		// 2. neighbouring nodes keep their spacing
		for k := 0; k < n; k++ {
			j := (k + 1) % n
			dx, dy := xs[j]-xs[k], ys[j]-ys[k]
			d := hypot(dx, dy)
			if d < 1e-4 {
				continue
			}
			c := (d - rest) / d * 0.5 * stiff
			xs[k] += dx * c
			ys[k] += dy * c
			xs[j] -= dx * c
			ys[j] -= dy * c
		}
		// 3. the skin resists sharp bends
		for k := 0; k < n; k++ {
			a, b := (k+n-1)%n, (k+1)%n
			xs[k] += ((xs[a]+xs[b])/2 - xs[k]) * bend
			ys[k] += ((ys[a]+ys[b])/2 - ys[k]) * bend
		}
		// 4. pressure keeps it inflated
		area := polyArea(xs, ys)
		push := (area0 - area) / (perim + 1) * 0.6
		for k := 0; k < n; k++ {
			a, b := (k+n-1)%n, (k+1)%n
			nx, ny := ys[b]-ys[a], -(xs[b] - xs[a]) // outward for clockwise-in-screen rings
			if d := hypot(nx, ny); d > 1e-4 {
				xs[k] += nx / d * push
				ys[k] += ny / d * push
			}
		}
	}
	for k, i := range p.skinIdx {
		w.X[i], w.Y[i] = xs[k], ys[k]
		w.collide(int(i))
		xs[k], ys[k] = w.X[i], w.Y[i]
	}

	// 5. liquid that ended up outside gets put back just inside the wall
	id := uint8(p.ID)
	for i := 0; i < w.N; i++ {
		if w.Own[i] != id || w.Role[i] > RoleNucleus || pointIn(xs, ys, w.X[i], w.Y[i]) {
			continue
		}
		bestD, bx, by := float32(1e9), w.X[i], w.Y[i]
		for k := 0; k < n; k++ {
			j := (k + 1) % n
			ex, ey := xs[j]-xs[k], ys[j]-ys[k]
			l2 := ex*ex + ey*ey
			t := float32(0)
			if l2 > 1e-6 {
				t = clamp(((w.X[i]-xs[k])*ex+(w.Y[i]-ys[k])*ey)/l2, 0, 1)
			}
			qx, qy := xs[k]+ex*t, ys[k]+ey*t
			if d := hypot(w.X[i]-qx, w.Y[i]-qy); d < bestD {
				bestD, bx, by = d, qx, qy
			}
		}
		// nudge inward toward the body centre
		dx, dy := cx-bx, cy-by
		if d := hypot(dx, dy); d > 1e-3 {
			bx += dx / d * 0.6
			by += dy / d * 0.6
		}
		w.X[i], w.Y[i] = bx, by
	}
}

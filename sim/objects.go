package sim

import "math"

// Objects are things in the room a plip can find out about. Nothing tells
// the plip what they do; it has to mess with them.
const (
	ObjBox  = iota // bump it and it sometimes spits out food
	ObjBall        // rolls and bounces
)

type Object struct {
	Kind   int
	Name   string
	X, Y   float32 // centre
	VX, VY float32
	HW, HH float32 // half extents (box) or radius in HW (ball)
	Spin   float32
	Held   bool // being dragged by the mouse

	// box internals
	cool, pending, buzz int
	Dispensed           int // total drops of food produced, ever
	lastBumper          *Plip
}

func NewBox(x float32) *Object  { return &Object{Kind: ObjBox, Name: "box", X: x, HW: 8, HH: 7} }
func NewBall(x float32) *Object { return &Object{Kind: ObjBall, Name: "ball", X: x, HW: 4, HH: 4} }

// Ready reports whether the box's lamp is lit.
func (o *Object) Ready() bool { return o.Kind == ObjBox && o.cool <= 0 }

// Moving reports whether the object is in motion.
func (o *Object) Moving() bool { return math.Abs(float64(o.VX))+math.Abs(float64(o.VY)) > 0.05 }

// Bump is a plip shoving the object.
func (o *Object) Bump(w *World, p *Plip, dir float32) {
	switch o.Kind {
	case ObjBox:
		if o.cool > 0 {
			return
		}
		o.cool = 120 * 4
		o.lastBumper = p
		if w.Rng.Float32() < 0.15 {
			o.buzz = 40 // sometimes it just rattles and gives nothing
			return
		}
		o.pending = 30 + w.Rng.Intn(70)
	case ObjBall:
		o.VX += dir * (0.9 + w.Rng.Float32()*0.6)
		o.VY -= 0.4 + w.Rng.Float32()*0.5
	}
}

// ObjectAt returns the object under (x, y).
func (w *World) ObjectAt(x, y float32) *Object {
	for _, o := range w.Objects {
		if x > o.X-o.HW-1 && x < o.X+o.HW+1 && y > o.Y-o.HH-1 && y < o.Y+o.HH+1 {
			return o
		}
	}
	return nil
}

func (w *World) stepObjects() {
	for _, o := range w.Objects {
		if o.Held {
			continue
		}
		o.VY += G
		if o.Kind == ObjBox {
			o.VX *= 0.8
		} else {
			o.VX *= 0.995
		}
		o.X += o.VX
		o.Y += o.VY
		if o.Y+o.HH > w.Floor+1 {
			o.Y = w.Floor + 1 - o.HH
			if o.Kind == ObjBall && o.VY > 0.4 {
				o.VY = -o.VY * 0.55
			} else {
				o.VY = 0
			}
			o.VX *= 0.97
		}
		if o.X-o.HW < 2 {
			o.X, o.VX = 2+o.HW, -o.VX*0.6
		} else if o.X+o.HW > w.W-2 {
			o.X, o.VX = w.W-2-o.HW, -o.VX*0.6
		}
		if o.Kind == ObjBall {
			o.Spin += o.VX / o.HW
		}
		if o.Kind == ObjBox {
			if o.cool > 0 {
				o.cool--
			}
			if o.buzz > 0 {
				o.buzz--
			}
			if o.pending > 0 {
				o.pending--
				if o.pending == 0 {
					w.dispense(o)
				}
			}
		}
	}
	// ball vs box
	for _, a := range w.Objects {
		for _, b := range w.Objects {
			if a.Kind == ObjBall && b.Kind == ObjBox {
				dx := a.X - clamp(a.X, b.X-b.HW, b.X+b.HW)
				dy := a.Y - clamp(a.Y, b.Y-b.HH, b.Y+b.HH)
				if d := hypot(dx, dy); d < a.HW && d > 1e-3 {
					a.X += dx / d * (a.HW - d)
					a.Y += dy / d * (a.HW - d)
					if math.Abs(float64(dx)) > math.Abs(float64(dy)) {
						a.VX = -a.VX * 0.6
					} else {
						a.VY = -a.VY * 0.5
					}
				}
			}
		}
	}
}

// dispense spits a little fountain of one colour out of the box's spout.
func (w *World) dispense(o *Object) {
	col := w.Rng.Intn(NCol)
	for k := 0; k < 10; k++ {
		vx := (w.Rng.Float32()*2 - 1) * 0.7
		w.loose(o.X+(w.Rng.Float32()-0.5)*1.5, o.Y-o.HH-2, col, vx, -1.2-w.Rng.Float32()*0.6)
	}
	o.Dispensed += 10
	w.FoodEvents = append(w.FoodEvents, FoodEvent{X: o.X, Tick: w.Tick, From: o})
}

// FoodEvent records food appearing, so plips can notice cause and effect.
type FoodEvent struct {
	X    float32
	Tick int
	From *Object // nil when dropped by the player
}

// pushOut keeps particle i out of solid objects. Plip bodies shove the ball.
func (w *World) pushOut(i int) {
	for _, o := range w.Objects {
		switch o.Kind {
		case ObjBox:
			x, y := w.X[i], w.Y[i]
			if x > o.X-o.HW && x < o.X+o.HW && y > o.Y-o.HH && y < o.Y+o.HH {
				// out through the nearest side, preferring the top
				dl, dr := x-(o.X-o.HW), (o.X+o.HW)-x
				dt := y - (o.Y - o.HH)
				switch {
				case dt <= dl && dt <= dr || w.PY[i] <= o.Y-o.HH:
					w.Y[i] = o.Y - o.HH
					w.X[i] = w.X[i]*0.7 + w.PX[i]*0.3
				case dl < dr:
					w.X[i] = o.X - o.HW
				default:
					w.X[i] = o.X + o.HW
				}
			}
		case ObjBall:
			dx, dy := w.X[i]-o.X, w.Y[i]-o.Y
			r := o.HW + 0.6
			if d := hypot(dx, dy); d < r && d > 1e-3 {
				w.X[i] = o.X + dx/d*r
				w.Y[i] = o.Y + dy/d*r
				if w.Own[i] != 0 && !o.Held {
					o.VX -= dx / d * 0.012
					o.VY -= dy / d * 0.006
				}
			}
		}
	}
}

// drawObjects paints the objects into an RGBA buffer.
func drawObjects(w *World, buf []byte, W, H int) {
	set := func(x, y int, c [3]float32) {
		if x < 0 || y < 0 || x >= W || y >= H {
			return
		}
		p := (y*W + x) * 4
		buf[p], buf[p+1], buf[p+2], buf[p+3] = u8(c[0]), u8(c[1]), u8(c[2]), 255
	}
	for _, o := range w.Objects {
		switch o.Kind {
		case ObjBox:
			x0, y0 := int(math.Round(float64(o.X-o.HW))), int(math.Round(float64(o.Y-o.HH)))
			x1, y1 := x0+int(o.HW*2), y0+int(o.HH*2)
			shake := 0
			if o.buzz > 0 && o.buzz%4 < 2 {
				shake = 1
			}
			for y := y0; y < y1; y++ {
				for x := x0; x < x1; x++ {
					c := [3]float32{84, 92, 86}
					switch {
					case y == y0:
						c = [3]float32{122, 130, 122}
					case y == y1-1 || x == x0 || x == x1-1:
						c = [3]float32{52, 58, 54}
					case (x+y)%7 == 0:
						c = [3]float32{80, 88, 82}
					}
					set(x+shake, y, c)
				}
			}
			// spout and slot
			cx := (x0 + x1) / 2
			for x := cx - 2; x <= cx+1; x++ {
				set(x+shake, y0-1, [3]float32{60, 66, 62})
				set(x+shake, y0-2, [3]float32{60, 66, 62})
			}
			set(cx-1+shake, y0-2, [3]float32{20, 22, 22})
			set(cx+shake, y0-2, [3]float32{20, 22, 22})
			// lamp
			lamp := [3]float32{40, 46, 40}
			switch {
			case o.pending > 0 && (o.pending/6)%2 == 0:
				lamp = [3]float32{230, 196, 90}
			case o.cool <= 0:
				lamp = [3]float32{120, 210, 130}
			}
			set(x0+2+shake, y0+2, lamp)
			set(x0+3+shake, y0+2, lamp)
			set(x0+2+shake, y0+3, lamp)
			set(x0+3+shake, y0+3, lamp)
			// a little grille
			for y := y0 + 6; y < y1-2; y += 2 {
				for x := cx - 3; x <= cx+3; x++ {
					set(x+shake, y, [3]float32{60, 66, 62})
				}
			}
		case ObjBall:
			r := o.HW
			for y := -int(r) - 1; y <= int(r)+1; y++ {
				for x := -int(r) - 1; x <= int(r)+1; x++ {
					fx, fy := float32(x)+0.5, float32(y)+0.5
					if fx*fx+fy*fy > r*r {
						continue
					}
					c := [3]float32{164, 82, 66}
					// a stripe that turns as it rolls
					a := float64(o.Spin)
					if math.Abs(float64(fx)*math.Cos(a)+float64(fy)*math.Sin(a)) < 0.9 {
						c = [3]float32{214, 196, 160}
					}
					if fx*fx+fy*fy > (r-1)*(r-1) {
						c = [3]float32{c[0] * 0.7, c[1] * 0.7, c[2] * 0.7}
					}
					if x == -2 && y == -2 {
						c = [3]float32{236, 220, 200}
					}
					set(int(o.X)+x, int(o.Y)+y, c)
				}
			}
		}
	}
}

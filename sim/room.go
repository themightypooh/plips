package sim

import (
	"math"
	"math/rand"
)

// Room layout in a 320x180 pixel world.
const (
	RoomW, RoomH, RoomFloor = 320, 180, 163
)

var RoomPlatforms = []Platform{
	{20, 84, 104},   // wall shelf
	{228, 292, 128}, // table top
}

// RoomBackground paints the pixel room: wallpaper, window, shelf, table,
// skirting board and floorboards. Returns opaque RGBA.
func RoomBackground() []byte {
	W, H, F := RoomW, RoomH, RoomFloor
	buf := make([]byte, W*H*4)
	rng := rand.New(rand.NewSource(7))
	set := func(x, y int, c [3]float32) {
		if x < 0 || y < 0 || x >= W || y >= H {
			return
		}
		p := (y*W + x) * 4
		buf[p], buf[p+1], buf[p+2], buf[p+3] = u8(c[0]), u8(c[1]), u8(c[2]), 255
	}
	rect := func(x0, y0, x1, y1 int, c [3]float32, grain float32) {
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				n := (rng.Float32() - 0.5) * grain
				set(x, y, [3]float32{c[0] + n, c[1] + n, c[2] + n})
			}
		}
	}
	// wallpaper: muted stripes, darker toward the corners
	for y := 0; y < F-8; y++ {
		for x := 0; x < W; x++ {
			c := [3]float32{66, 72, 70}
			if (x/6)%2 == 0 {
				c = [3]float32{62, 68, 66}
			}
			vx := float32(x-W/2) / float32(W/2)
			vy := float32(y-60) / 120
			v := 1 - 0.28*(vx*vx+vy*vy)
			n := (rng.Float32() - 0.5) * 5
			set(x, y, [3]float32{c[0]*v + n, c[1]*v + n, c[2]*v + n})
		}
	}
	// window with a night sky
	wx0, wy0, wx1, wy1 := 124, 26, 196, 86
	rect(wx0-3, wy0-3, wx1+3, wy1+4, [3]float32{92, 80, 66}, 8)
	for y := wy0; y < wy1; y++ {
		t := float32(y-wy0) / float32(wy1-wy0)
		for x := wx0; x < wx1; x++ {
			set(x, y, [3]float32{18 + t*16, 24 + t*20, 40 + t*22})
		}
	}
	for i := 0; i < 14; i++ {
		set(wx0+2+rng.Intn(wx1-wx0-4), wy0+2+rng.Intn(30), [3]float32{170, 176, 190})
	}
	for y := -4; y <= 4; y++ { // moon
		for x := -4; x <= 4; x++ {
			if x*x+y*y <= 14 && (x+2)*(x+2)+(y-1)*(y-1) > 10 {
				set(wx0+16+x, wy0+14+y, [3]float32{214, 210, 186})
			}
		}
	}
	rect((wx0+wx1)/2-1, wy0, (wx0+wx1)/2+1, wy1, [3]float32{92, 80, 66}, 6)
	rect(wx0, (wy0+wy1)/2-1, wx1, (wy0+wy1)/2+1, [3]float32{92, 80, 66}, 6)
	rect(wx0-5, wy1+3, wx1+5, wy1+6, [3]float32{104, 92, 76}, 6) // sill
	// soft light pool under the window
	for y := wy1 + 6; y < F; y++ {
		for x := wx0 - 30; x < wx1+30; x++ {
			if x < 0 || x >= W {
				continue
			}
			p := (y*W + x) * 4
			dx := float64(x-(wx0+wx1)/2) / 60
			k := float32(0.06 * math.Max(0, 1-dx*dx) * float64(1-float32(y-wy1)/float32(F-wy1)))
			for c := 0; c < 3; c++ {
				buf[p+c] = u8(float32(buf[p+c]) * (1 + k))
			}
		}
	}
	// shelf with brackets
	sh := RoomPlatforms[0]
	rect(int(sh.X0), int(sh.Y)+1, int(sh.X1), int(sh.Y)+4, [3]float32{110, 86, 62}, 12)
	rect(int(sh.X0)+6, int(sh.Y)+4, int(sh.X0)+8, int(sh.Y)+10, [3]float32{58, 58, 60}, 4)
	rect(int(sh.X1)-8, int(sh.Y)+4, int(sh.X1)-6, int(sh.Y)+10, [3]float32{58, 58, 60}, 4)
	// table
	tb := RoomPlatforms[1]
	rect(int(tb.X0), int(tb.Y)+1, int(tb.X1), int(tb.Y)+5, [3]float32{104, 76, 54}, 12)
	rect(int(tb.X0), int(tb.Y)+5, int(tb.X1), int(tb.Y)+6, [3]float32{72, 52, 38}, 6)
	rect(int(tb.X0)+4, int(tb.Y)+6, int(tb.X0)+7, F+1, [3]float32{88, 64, 46}, 10)
	rect(int(tb.X1)-7, int(tb.Y)+6, int(tb.X1)-4, F+1, [3]float32{88, 64, 46}, 10)
	// skirting board
	rect(0, F-8, W, F+1, [3]float32{150, 146, 136}, 6)
	rect(0, F-8, W, F-7, [3]float32{176, 172, 162}, 4)
	// floorboards
	for y := F + 1; y < H; y++ {
		row := (y - F - 1) / 4
		for x := 0; x < W; x++ {
			c := [3]float32{92, 70, 50}
			if row%2 == 1 {
				c = [3]float32{86, 65, 47}
			}
			n := (rng.Float32()-0.5)*10 + float32(math.Sin(float64(x)*0.21+float64(row)*3))*3
			if (y-F-1)%4 == 0 || (x+row*37)%58 == 0 {
				c = [3]float32{60, 45, 33}
			}
			set(x, y, [3]float32{c[0] + n, c[1] + n, c[2] + n})
		}
	}
	return buf
}

// CellBackground is a plain dark specimen cell for the gallery.
func CellBackground(w, h, floor int) []byte {
	buf := make([]byte, w*h*4)
	rng := rand.New(rand.NewSource(3))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			p := (y*w + x) * 4
			c := [3]float32{24, 28, 30}
			if y > floor {
				c = [3]float32{40, 44, 44}
				if y == floor+1 {
					c = [3]float32{62, 68, 68}
				}
			} else {
				c[0] += float32(y) * 0.06
				c[1] += float32(y) * 0.07
				c[2] += float32(y) * 0.07
			}
			n := (rng.Float32() - 0.5) * 3
			buf[p], buf[p+1], buf[p+2], buf[p+3] = u8(c[0]+n), u8(c[1]+n), u8(c[2]+n), 255
		}
	}
	return buf
}

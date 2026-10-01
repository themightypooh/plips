package main

import (
	"bytes"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font/gofont/goregular"
)

// A tiny immediate-mode UI: buttons are declared during Update (which also
// reports clicks) and drawn later in Draw.

var (
	colBar      = color.RGBA{18, 21, 23, 255}
	colLine     = color.RGBA{40, 46, 49, 255}
	colBtn      = color.RGBA{30, 35, 38, 255}
	colBtnHover = color.RGBA{42, 49, 53, 255}
	colBtnOn    = color.RGBA{58, 74, 68, 255}
	colText     = color.RGBA{206, 214, 209, 255}
	colDim      = color.RGBA{128, 140, 134, 255}
	colAccent   = color.RGBA{150, 194, 172, 255}
)

var fontSrc *text.GoTextFaceSource

func face(size float64) *text.GoTextFace {
	if fontSrc == nil {
		s, err := text.NewGoTextFaceSource(bytes.NewReader(goregular.TTF))
		if err != nil {
			panic(err)
		}
		fontSrc = s
	}
	return &text.GoTextFace{Source: fontSrc, Size: size}
}

func drawText(dst *ebiten.Image, s string, x, y float64, size float64, c color.Color) {
	op := &text.DrawOptions{}
	op.GeoM.Translate(x, y)
	op.ColorScale.ScaleWithColor(c)
	text.Draw(dst, s, face(size), op)
}

func textWidth(s string, size float64) float64 {
	w, _ := text.Measure(s, face(size), 0)
	return w
}

type rect struct{ X, Y, W, H float64 }

func (r rect) has(x, y float64) bool { return x >= r.X && y >= r.Y && x < r.X+r.W && y < r.Y+r.H }

type widget struct {
	r        rect
	label    string
	on       bool
	disabled bool
	swatch   color.Color // non-nil: a colour button
}

type UI struct {
	mx, my  float64
	click   bool
	widgets []widget
	panels  []rect
}

func (u *UI) Begin() {
	x, y := ebiten.CursorPosition()
	u.mx, u.my = float64(x), float64(y)
	u.click = inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	u.widgets = u.widgets[:0]
	u.panels = u.panels[:0]
}

// Panel marks a bar area; clicks inside it never reach the play area.
func (u *UI) Panel(r rect) { u.panels = append(u.panels, r) }

// OverUI reports whether the cursor is on a bar.
func (u *UI) OverUI() bool {
	for _, p := range u.panels {
		if p.has(u.mx, u.my) {
			return true
		}
	}
	return false
}

const btnH = 30

// ButtonWidth is how wide Button will make a label.
func ButtonWidth(label string) float64 { return textWidth(label, 14) + 24 }

// Button declares a button at (x, y) and reports whether it was clicked.
func (u *UI) Button(x, y float64, label string, on, disabled bool) (bool, float64) {
	r := rect{x, y, ButtonWidth(label), btnH}
	u.widgets = append(u.widgets, widget{r: r, label: label, on: on, disabled: disabled})
	return !disabled && u.click && r.has(u.mx, u.my), r.W
}

// Swatch declares a square colour button.
func (u *UI) Swatch(x, y float64, c color.Color, on bool) bool {
	r := rect{x, y, btnH, btnH}
	u.widgets = append(u.widgets, widget{r: r, on: on, swatch: c})
	return u.click && r.has(u.mx, u.my)
}

func (u *UI) Draw(dst *ebiten.Image) {
	for _, p := range u.panels {
		vector.FillRect(dst, float32(p.X), float32(p.Y), float32(p.W), float32(p.H), colBar, false)
		vector.StrokeLine(dst, float32(p.X), float32(p.Y)+0.5, float32(p.X+p.W), float32(p.Y)+0.5, 1, colLine, false)
		vector.StrokeLine(dst, float32(p.X), float32(p.Y+p.H)-0.5, float32(p.X+p.W), float32(p.Y+p.H)-0.5, 1, colLine, false)
	}
	for _, w := range u.widgets {
		r := w.r
		hover := r.has(u.mx, u.my) && !w.disabled
		x, y, ww, hh := float32(r.X), float32(r.Y), float32(r.W), float32(r.H)
		if w.swatch != nil {
			vector.FillRect(dst, x+3, y+3, ww-6, hh-6, w.swatch, false)
			if w.on {
				vector.StrokeRect(dst, x+0.5, y+0.5, ww-1, hh-1, 2, colAccent, false)
			} else if hover {
				vector.StrokeRect(dst, x+1, y+1, ww-2, hh-2, 1, colDim, false)
			}
			continue
		}
		bg := colBtn
		if w.on {
			bg = colBtnOn
		} else if hover {
			bg = colBtnHover
		}
		vector.FillRect(dst, x, y, ww, hh, bg, false)
		vector.StrokeRect(dst, x+0.5, y+0.5, ww-1, hh-1, 1, colLine, false)
		c := colText
		if w.disabled {
			c = color.RGBA{80, 88, 84, 255}
		} else if w.on {
			c = colAccent
		}
		drawText(dst, w.label, r.X+12, r.Y+7, 14, c)
	}
}

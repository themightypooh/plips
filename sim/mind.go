package sim

import (
	"fmt"
	"math"
	"time"
)

// DiaryEntry is something worth telling the player about.
type DiaryEntry struct {
	Unix int64  `json:"t"`
	Text string `json:"text"`
}

func (p *Plip) note(w *World, text string) {
	p.Diary = append(p.Diary, DiaryEntry{Unix: time.Now().Unix(), Text: p.Name + " " + text})
	if len(p.Diary) > 80 {
		p.Diary = p.Diary[len(p.Diary)-80:]
	}
	p.lastNote = w.Tick
}

func (p *Plip) bad() float32 {
	var s float32
	for i, v := range p.Drives {
		s += v * driveWeight[i]
	}
	return s
}

func (p *Plip) drift(w *World) {
	d := &p.Drives
	d[Hunger] += 0.000022 * p.G.Metab
	d[Boredom] += 0.00006
	d[Tired] += 0.000015
	if p.Moving {
		d[Tired] += 0.00002
	}
	if o := w.Other(p); o != nil && math.Abs(float64(o.MX-p.MX)) < 60 {
		d[Lonely] -= 0.0001
	} else if w.Hand.Visible && math.Abs(float64(w.Hand.X-p.MX)) < 40 {
		d[Lonely] -= 0.00008 // you being near counts as company
	} else {
		d[Lonely] += 0.00002
	}
	d[Pain] *= 0.998
	for i := range d {
		d[i] = clamp01(d[i])
	}
}

// Ate is called for each food particle absorbed.
func (p *Plip) Ate(col int) {
	if col < 0 || col >= NCol {
		return
	}
	p.Drives[Hunger] = clamp01(p.Drives[Hunger] - 0.02*p.G.Taste[col])
	p.Eaten++
}

func (p *Plip) Hurt(x float32) {
	p.Drives[Pain] = clamp01(p.Drives[Pain] + x)
	p.Flinch = 20
}

// Pet: the hand was nice to it. It counts toward whatever it's doing.
func (p *Plip) Pet() {
	p.Happy = 50
	if p.Brain == nil {
		if p.Hop == 0 {
			p.Hop = 1
		}
		return
	}
	p.out[OPetted] = 1
	p.Drives[Boredom] = clamp01(p.Drives[Boredom] - 0.1)
	p.Drives[Lonely] = clamp01(p.Drives[Lonely] - 0.15)
}

// Poke hurts a little and knocks some of it loose.
func (p *Plip) Poke(w *World, x, y float32) {
	for i := 0; i < w.N; i++ {
		if w.Own[i] != uint8(p.ID) || w.Role[i] == RoleLimb {
			continue
		}
		dx, dy := w.X[i]-x, w.Y[i]-y
		d := hypot(dx, dy)
		if d < 9 {
			f := (1-d/9)*2.4 + 0.3
			w.X[i] += dx / (d + 0.01) * f * 0.5
			w.Y[i] += dy/(d+0.01)*f*0.5 - f*0.4
			if d < 5 && w.Role[i] == RoleBody && w.Rng.Float32() < 0.4 && p.Mass > MinMass {
				w.Own[i] = 0 // knocked loose; it slurps its own goo back up later
				w.Age[i] = 0
			}
		}
	}
	p.Hurt(0.3)
	p.Flinch = 40
	if p.Hop == 0 {
		p.Hop = 1
	}
	if p.Brain != nil {
		p.out[OPoked] = 1
	}
}

// Doing describes the current action for the HUD.
func (p *Plip) Doing(w *World) string {
	if p.Brain == nil || p.actMax == 0 {
		return ""
	}
	return p.ActText(p.Act, p.Tgt)
}

func (p *Plip) ActText(a, t int) string {
	switch {
	case a == ARest:
		return "rest"
	case a == APlay && t == TNone:
		return "bounce around"
	}
	return ActNames[a] + " " + p.TargetName(t)
}

func (p *Plip) TargetName(t int) string {
	switch {
	case t < NCol:
		return "the " + lower(Colours[t].Name)
	case t == TBox:
		return "the box"
	case t == TBall:
		return "the ball"
	case t == THand:
		return "you"
	}
	return ""
}

func lower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

func (w *World) object(kind int) *Object {
	for _, o := range w.Objects {
		if o.Kind == kind {
			return o
		}
	}
	return nil
}

// food scans the floor for the nearest loose puddle of each colour.
func (p *Plip) food(w *World) (pos [NCol]float32, n [NCol]int) {
	var best [NCol]float32
	for i := range best {
		best[i] = 1e9
	}
	for i := 0; i < w.N; i++ {
		if w.Own[i] != 0 || w.Mat[i] == 0 || w.Y[i] < w.Floor-14 {
			continue
		}
		c := int(w.Mat[i]) - 1
		n[c]++
		if d := float32(math.Abs(float64(w.X[i] - p.CoreX))); d < best[c] {
			best[c], pos[c] = d, w.X[i]
		}
	}
	return
}

// look builds what the plip perceives about every target, and where they are.
func (p *Plip) look(w *World) (s Situation, pos [NT]float32, ok [NT]bool) {
	s.Drives = p.Drives
	fp, fn := p.food(w)
	for c := 0; c < NCol; c++ {
		if fn[c] > 0 {
			pos[c], ok[c] = fp[c], true
			s.State[c][1] = clamp01(float32(fn[c]) / 30)
		}
	}
	if o := w.object(ObjBox); o != nil {
		pos[TBox], ok[TBox] = o.X, true
		if o.Ready() {
			s.State[TBox][1] = 1
		}
	}
	if o := w.object(ObjBall); o != nil {
		pos[TBall], ok[TBall] = o.X, true
		if o.Moving() {
			s.State[TBall][1] = 1
		}
	}
	if w.Hand.Visible {
		pos[THand], ok[THand] = w.Hand.X, true
		s.State[THand][1] = 1
	}
	pos[TNone], ok[TNone] = p.CoreX, true
	for t := 0; t < NT; t++ {
		if ok[t] {
			s.State[t][0] = clamp01(float32(math.Abs(float64(pos[t]-p.CoreX))) / 120)
			s.State[t][2] = p.Brain.Novel[t]
			s.State[t][3] = p.recentSurp[t]
		}
	}
	return
}

func validActs(t int) []int {
	switch {
	case t < NCol:
		return []int{AApproach, AEat, ARetreat}
	case t == TBox, t == TBall:
		return []int{AApproach, APush, APlay, AWatch, ARetreat}
	case t == THand:
		return []int{AApproach, APlay, AWatch, ARetreat}
	}
	return []int{ARest, APlay}
}

func (p *Plip) decide(w *World) {
	sit, _, ok := p.look(w)
	type opt struct{ t, a int }
	var opts []opt
	for t := 0; t < NT; t++ {
		if ok[t] {
			for _, a := range validActs(t) {
				opts = append(opts, opt{t, a})
			}
		}
	}
	scores := make([]float32, len(opts))
	xs := make([][]float32, len(opts))
	best := float32(math.Inf(-1))
	for i, o := range opts {
		xs[i] = p.Brain.input(&sit, o.t, o.a)
		q, _ := p.Brain.Q(xs[i])
		if q > best {
			best = q
		}
		scores[i] = q + p.instinct(&sit, o.t, o.a)
	}
	// learn from the last choice now we know what the situation turned into
	if p.prevX != nil {
		p.Brain.Train(p.prevX, p.prevR+gamma*best, p.G.LR)
	}
	k := softmaxPick(w.Rng, scores, p.G.Temp)
	c := opts[k]
	p.Act, p.Tgt, p.x, p.sit = c.a, c.t, xs[k], sit
	p.Expect = p.Brain.Predict(&sit, c.t, c.a)
	p.Brain.Novel[c.t] *= 0.85
	p.startBad, p.startHunger, p.startPain = p.bad(), p.Drives[Hunger], p.Drives[Pain]
	p.startTick = w.Tick
	p.actT, p.bumped = 0, false
	p.out = [NO]float32{}
	p.actMax = [NA]int{300, 420, 260, 360, 240, 180, 420}[c.a]
	p.tgtOK = true
	_, pos, _ := p.look(w)
	p.tgtX, p.tgtStartX = pos[c.t], pos[c.t]
}

// instinct is the inborn nudge added on top of what it has learned: eat
// when hungry, rest when tired, and be drawn to things it hasn't explored.
func (p *Plip) instinct(s *Situation, t, a int) float32 {
	d := s.Drives
	var v float32
	switch {
	case a == AEat && t < NCol:
		v += 1.2 * d[Hunger] * (1 - s.State[t][0])
	case a == ARest:
		v += 1.2*d[Tired] - 0.3
	}
	if t != TNone && a != ARetreat {
		v += 0.35 * p.G.Curious * p.Brain.Novel[t] * d[Boredom]
	}
	return v
}

func (p *Plip) think(w *World) {
	if p.actMax == 0 {
		p.decide(w)
	}
	p.actT++
	if p.actT%15 == 1 {
		_, pos, ok := p.look(w)
		p.tgtX, p.tgtOK = pos[p.Tgt], ok[p.Tgt]
	}
	var obj *Object
	switch p.Tgt {
	case TBox:
		obj = w.object(ObjBox)
	case TBall:
		obj = w.object(ObjBall)
	case THand:
		p.tgtX = w.Hand.X
	}
	if obj != nil {
		p.tgtX = obj.X
	}
	reach := p.Rad*p.ax + 4
	if obj != nil {
		reach = p.Rad*p.ax + obj.HW + 2
	}
	near := float32(math.Abs(float64(p.CoreX-p.tgtX))) < reach

	done := p.actT >= p.actMax || !p.tgtOK
	switch p.Act {
	case AApproach:
		p.TX = p.tgtX
		done = done || near
	case AEat:
		p.TX = p.tgtX
		if p.Tgt < NCol {
			p.EatCol = p.Tgt
		}
	case APush:
		dir := sign(p.tgtX - p.CoreX)
		p.TX = p.tgtX + dir*4 // lean into it
		if near && !p.bumped && obj != nil {
			obj.Bump(w, p, dir)
			p.bumped = true
			if p.actT < p.actMax-80 {
				p.actT = p.actMax - 80 // hang around a moment to see what happens
			}
		}
	case APlay:
		if p.Tgt == TNone {
			if p.actT%90 == 1 {
				p.TX = p.Rad + 4 + w.Rng.Float32()*(w.W-2*p.Rad-8)
			}
			if p.Hop == 0 && w.Rng.Float32() < 0.01 {
				p.Hop = 1
			}
			p.Drives[Boredom] = clamp01(p.Drives[Boredom] - 0.0006)
			break
		}
		p.TX = p.tgtX + float32(math.Sin(float64(p.actT)*0.04))*12
		if near {
			if p.Hop == 0 && w.Rng.Float32() < 0.015 {
				p.Hop = 1
			}
			if obj != nil && obj.Kind == ObjBall && w.Rng.Float32() < 0.01 {
				obj.Bump(w, p, sign(p.tgtX-p.CoreX))
			}
			p.Drives[Boredom] = clamp01(p.Drives[Boredom] - 0.0008)
		}
	case AWatch:
		p.TX = p.CoreX
		if math.Abs(float64(p.tgtX-p.CoreX)) > 1 {
			p.Face = sign(p.tgtX - p.CoreX)
		}
	case ARetreat:
		if p.actT == 1 {
			p.TX = p.CoreX - sign(p.tgtX-p.CoreX)*50
		}
	case ARest:
		p.TX = p.CoreX
		p.Resting = true
		p.Drives[Tired] = clamp01(p.Drives[Tired] - 0.0015)
	}
	if done {
		p.finish(w, obj)
	}
}

// finish looks at what happened, gets surprised (or not), learns, and maybe
// writes in the diary.
func (p *Plip) finish(w *World, obj *Object) {
	b := p.Brain
	t, a := p.Tgt, p.Act
	byBox := false
	for _, e := range w.FoodEvents {
		if e.Tick >= p.startTick && math.Abs(float64(e.X-p.CoreX)) < 70 {
			p.out[OFood] = 1
			if e.From != nil && e.From.Kind == ObjBox && e.From.lastBumper == p {
				byBox = true
			}
		}
	}
	switch {
	case obj != nil:
		if math.Abs(float64(obj.X-p.tgtStartX)) > 3 {
			p.out[OMoved] = 1
		}
	case t == THand:
		if math.Abs(float64(w.Hand.X-p.tgtStartX)) > 12 {
			p.out[OMoved] = 1
		}
	}
	p.out[OFed] = clamp01((p.startHunger - p.Drives[Hunger]) * 5)
	p.out[OHurt] = clamp01((p.Drives[Pain] - p.startPain) * 3)

	surprise := b.Surprise(&p.sit, t, a, p.out)
	p.Drives[Boredom] = clamp01(p.Drives[Boredom] - surprise*0.5)
	curiosity := p.G.Curious * surprise * 1.5
	reward := 2 * ((p.startBad-p.bad())*3 + curiosity + p.out[OPetted] - p.out[OPoked])
	p.prevX, p.prevR = p.x, reward
	p.Stats[t][a].N++
	p.Stats[t][a].Reward += reward
	p.Stats[t][a].Surprise += surprise
	p.LastReward, p.LastSurprise = reward, surprise
	for i := range p.recentSurp {
		p.recentSurp[i] *= 0.7
	}
	p.recentSurp[t] = clamp01(surprise * 2)
	if surprise > 0.05 {
		p.SurpriseAt = t
	}
	b.Remember(a, t, p.out)
	p.actMax = 0
	p.diary(w, t, a, surprise, byBox)
}

func (p *Plip) diary(w *World, t, a int, surprise float32, byBox bool) {
	b := p.Brain
	if p.Seen == nil {
		p.Seen = map[string]bool{}
	}
	once := func(key, text string) {
		if !p.Seen[key] {
			p.Seen[key] = true
			p.note(w, text)
		}
	}
	if t == TBox || t == TBall {
		if a == APush || a == APlay {
			once(fmt.Sprintf("try%d%d", t, a), "had a go at "+p.ActText(a, t)+" for the first time")
		}
	}
	if byBox {
		p.BoxFood++
		once("boxfood", "got food out of the box!")
		if p.BoxFood == 5 {
			p.note(w, "keeps getting food out of the box. It's onto something")
		}
	}
	if b.Tried[t][a] > 6 && b.Err[t][a] < 0.03 && (t == TBox || t == TBall) && (a == APush || a == APlay) {
		once(fmt.Sprintf("know%d%d", t, a), "has figured out what happens when it tries to "+p.ActText(a, t))
	}
	if surprise > 0.3 && w.Tick-p.lastNote > 120*60 {
		top, tv := -1, float32(0.2)
		for o := range p.out {
			if d := p.out[o] - p.Expect[o]; d > tv {
				top, tv = o, d
			}
		}
		if top >= 0 {
			p.note(w, "didn't expect "+OutcomeNames[top]+" after trying to "+p.ActText(a, t))
		}
	}
	if p.out[OPetted] > 0 {
		if p.petTeach == nil {
			p.petTeach = map[[2]int]int{}
		}
		k := [2]int{t, a}
		p.petTeach[k]++
		if p.petTeach[k] == 3 {
			p.note(w, "is learning that you like it when it tries to "+p.ActText(a, t))
		}
	}
	if a == AEat && t < NCol { // has its favourite food changed?
		fav, fv, cur := -1, float32(-1e9), float32(-1e9)
		for c := 0; c < NCol; c++ {
			if b.Tried[c][AEat] < 5 {
				continue
			}
			v := b.Liking(&p.sit, c)
			if v > fv {
				fav, fv = c, v
			}
			if c == p.favourite {
				cur = v
			}
		}
		if fav >= 0 && fav != p.favourite && fv > cur+0.15 && w.Tick-p.favAt > 120*600 {
			if p.favourite >= 0 {
				p.note(w, "has started to prefer "+lower(Colours[fav].Name))
			} else {
				p.note(w, "seems to like "+lower(Colours[fav].Name)+" best")
			}
			p.favourite, p.favAt = fav, w.Tick
		}
	}
}

// Ideas lists what the brain currently rates best, for the mind panel.
func (p *Plip) Ideas(w *World, n int) []string {
	if p.Brain == nil {
		return nil
	}
	sit, _, ok := p.look(w)
	type kv struct {
		s string
		v float32
	}
	var l []kv
	for t := 0; t < NT; t++ {
		if !ok[t] {
			continue
		}
		for _, a := range validActs(t) {
			q, _ := p.Brain.Q(p.Brain.input(&sit, t, a))
			l = append(l, kv{p.ActText(a, t), q})
		}
	}
	for i := 0; i < len(l); i++ {
		for j := i + 1; j < len(l); j++ {
			if l[j].v > l[i].v {
				l[i], l[j] = l[j], l[i]
			}
		}
	}
	var out []string
	for i := 0; i < n && i < len(l); i++ {
		out = append(out, l[i].s)
	}
	return out
}

// Expecting says what it thinks will happen from what it's doing now.
func (p *Plip) Expecting() string {
	if p.Brain == nil || p.actMax == 0 {
		return ""
	}
	top, tv := -1, float32(0.25)
	for o, v := range p.Expect {
		if v > tv {
			top, tv = o, v
		}
	}
	if top < 0 {
		return "not sure what will happen"
	}
	return "expects " + OutcomeNames[top]
}

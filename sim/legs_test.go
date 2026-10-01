package sim

import (
	"math/rand"
	"testing"
)

func leggedGenome(r *rand.Rand) Genome {
	g := RandomGenome(r)
	g.Limbs = []Limb{{Kind: LimbLeg, Len: 20, Stiff: 0.6, Tip: true, Mirror: true}}
	g.Mass = 100
	return g
}

// walkSpeed runs a plip back and forth across the room and returns its
// average speed (px/tick) in each window.
func walkSpeed(p *Plip, w *World, windows, ticks int) []float32 {
	var out []float32
	tgt := float32(40)
	for k := 0; k < windows; k++ {
		var dist float32
		var moving int
		for t := 0; t < ticks; t++ {
			if d := p.CoreX - tgt; d*d < 25 {
				if tgt < 100 {
					tgt = w.W - 40
				} else {
					tgt = 40
				}
			}
			p.wanderT = 1 << 30
			p.TX = tgt
			x, dir := p.CoreX, sign(tgt-p.CoreX)
			w.Step()
			if p.Moving {
				moving++
				dist += (p.CoreX - x) * dir
			}
		}
		out = append(out, dist/float32(max(moving, 1)))
	}
	return out
}

func TestLegsLearnToWalk(t *testing.T) {
	better := 0
	for seed := int64(1); seed <= 4; seed++ {
		r := rand.New(rand.NewSource(seed))
		w := NewWorld(RoomW, RoomH, RoomFloor, seed)
		p := NewPlip(leggedGenome(r), "", false)
		w.AddPlip(p, 100, nil)
		p.Motor.Strength = 1 // strong legs; this is about finding a gait
		sp := walkSpeed(p, w, 12, 120*30)
		t.Logf("seed %d ooze max %.3f speeds %.3f skill %.3f %s", seed, 0.2*p.G.Speed, sp, p.Motor.Skill, p.WalkWords())
		if sp[len(sp)-1] > sp[0]*1.3 {
			better++
		}
	}
	if better < 4 {
		t.Fatalf("only %d of 4 plips got faster at walking", better)
	}
}

func TestLimbsGrowFromBody(t *testing.T) {
	r := rand.New(rand.NewSource(3))
	w := NewWorld(RoomW, RoomH, RoomFloor, 3)
	p := NewPlip(leggedGenome(r), "x", true)
	p.Bud()
	w.AddPlip(p, 100, nil)
	w.Step()
	m0 := p.Mass
	for i := 0; i < growPeriod*3; i++ {
		w.Step()
	}
	g := p.LimbGrowth()
	if g[0] < 3 {
		t.Fatalf("legs didn't grow: %v", g)
	}
	if p.Mass >= m0 {
		t.Fatalf("growing should use body liquid: %d -> %d", m0, p.Mass)
	}
	for li := range p.limbs {
		for s, i := range p.limbIdx[li] {
			if (s < p.limbs[li].grown) != (i >= 0) {
				t.Fatalf("limb %d seg %d: grown %d but idx %d", li, s, p.limbs[li].grown, i)
			}
		}
	}
}

func TestLegsGainStrengthWithUse(t *testing.T) {
	r := rand.New(rand.NewSource(5))
	w := NewWorld(RoomW, RoomH, RoomFloor, 5)
	p := NewPlip(leggedGenome(r), "", false)
	w.AddPlip(p, 100, nil)
	walkSpeed(p, w, 1, 120*20)
	if p.legLift > p.Rad*0.1 {
		t.Fatalf("weak new legs lifted the body %.2f px", p.legLift)
	}
	walkSpeed(p, w, 1, 120*60*12)
	t.Logf("strength %.2f lift %.2f rad %.2f", p.Strength(), p.legLift, p.Rad)
	if p.Strength() < 0.2 || p.Strength() > 0.9 {
		t.Fatalf("strength after 12 min of walking: %.2f", p.Strength())
	}
}

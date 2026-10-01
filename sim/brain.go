package sim

import (
	"math"
	"math/rand"
)

// Drives are the plip's internal chemistry, each 0..1. Reducing them is what
// the brain learns to want.
const (
	Hunger = iota
	Pain
	Boredom
	Lonely
	Tired
	NDrive
)

var DriveNames = [NDrive]string{"hunger", "pain", "bored", "lonely", "tired"}

// how much each drive counts toward feeling bad
var driveWeight = [NDrive]float32{1, 2, 0.6, 0.5, 0.6}

// Attention targets: one per food colour, then these.
const (
	TOther = NCol + iota
	THand
	TNone
	NT
)

// Actions.
const (
	AApproach = iota
	AEat
	ARetreat
	APlay
	ARest
	AHop
	NA
)

var ActNames = [NA]string{"approach", "eat", "back off from", "play with", "rest", "hop"}

// Features: drives, target one-hot, drive x target, bias.
const (
	fDrive = 0
	fTgt   = fDrive + NDrive
	fCross = fTgt + NT
	fBias  = fCross + NT*NDrive
	NF     = fBias + 1
)

// Brain is a single learned layer mapping (how I feel, what I'm looking at)
// to how good each action seems. The drive x target features let it learn
// things like "eat blue when hungry" without a hidden layer.
type Brain struct {
	W [NA][NF]float32
}

func cross(t, d int) int { return fCross + t*NDrive + d }

// NewBrain wires up weak instincts from the genome; everything else is learned.
func NewBrain(g Genome) *Brain {
	r := rand.New(rand.NewSource(g.Instinct))
	b := &Brain{}
	for a := range b.W {
		for f := range b.W[a] {
			b.W[a][f] = (r.Float32()*2 - 1) * 0.08
		}
	}
	for t := 0; t < NCol; t++ {
		b.W[AEat][cross(t, Hunger)] += 0.7
		b.W[AApproach][cross(t, Boredom)] += 0.2
	}
	b.W[AEat][cross(TOther, Hunger)] += 0.1
	b.W[ARest][cross(TNone, Tired)] += 1.2
	b.W[AHop][cross(TNone, Boredom)] += 0.35
	b.W[APlay][cross(TOther, Lonely)] += 0.7
	b.W[APlay][cross(TOther, Boredom)] += 0.35
	b.W[APlay][cross(THand, Boredom)] += 0.25
	b.W[ARetreat][cross(TOther, Pain)] += 0.4
	b.W[ARetreat][cross(THand, Pain)] += 0.4
	return b
}

func Features(d [NDrive]float32, t int) (f [NF]float32) {
	copy(f[fDrive:], d[:])
	f[fTgt+t] = 1
	for i := 0; i < NDrive; i++ {
		f[cross(t, i)] = d[i]
	}
	f[fBias] = 1
	return
}

func (b *Brain) Score(a int, f *[NF]float32) float32 {
	var s float32
	for i, v := range f {
		if v != 0 {
			s += b.W[a][i] * v
		}
	}
	return s
}

// Learn nudges the action's weights toward (reward > 0) or away from
// (reward < 0) the situation it was chosen in.
func (b *Brain) Learn(a int, f *[NF]float32, reward, lr float32) {
	for i, v := range f {
		if v == 0 {
			continue
		}
		w := b.W[a][i] + lr*reward*v
		if w > 4 {
			w = 4
		} else if w < -4 {
			w = -4
		}
		b.W[a][i] = w
	}
}

// Liking is how keen the brain is to eat colour c when fairly hungry.
func (b *Brain) Liking(c int) float32 {
	var d [NDrive]float32
	d[Hunger] = 0.7
	f := Features(d, c)
	return b.Score(AEat, &f)
}

func softmaxPick(r *rand.Rand, scores []float32, temp float32) int {
	max := float32(math.Inf(-1))
	for _, s := range scores {
		if s > max {
			max = s
		}
	}
	var sum float64
	ps := make([]float64, len(scores))
	for i, s := range scores {
		ps[i] = math.Exp(float64((s - max) / temp))
		sum += ps[i]
	}
	x := r.Float64() * sum
	for i, p := range ps {
		if x < p {
			return i
		}
		x -= p
	}
	return len(scores) - 1
}

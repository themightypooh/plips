package sim

import (
	"math"
	"math/rand"
)

// Drives are the plip's internal chemistry, each 0..1. Bringing them down
// is what feels good.
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
var driveWeight = [NDrive]float32{1, 2, 0.7, 0.4, 0.6}

// Attention targets: the nearest puddle of each food colour, then these.
const (
	TBox = NCol + iota
	TBall
	THand
	TNone
	NT
)

// Actions.
const (
	AApproach = iota
	AEat
	APush
	APlay
	AWatch
	ARetreat
	ARest
	NA
)

var ActNames = [NA]string{"go to", "eat", "push", "play with", "watch", "back off from", "rest"}

// Outcomes: what the plip notices happened during an action. The forward
// model learns to predict these; being wrong is surprising.
const (
	OFood   = iota // food appeared nearby
	OMoved         // the target moved
	OFed           // hunger went down
	OHurt          // pain went up
	OPetted        // the hand petted it
	OPoked         // the hand poked it
	NO
)

var OutcomeNames = [NO]string{"food", "it moving", "a full belly", "pain", "a pet", "a poke"}

const (
	nMem    = 16 // echo memory size
	nMemIn  = NA + NT + NO
	nState  = 4 // per-target state: distance, active, novelty, recently surprising
	nIn     = NDrive + NT + NA + nState + nMemIn + nMem + 1
	nHidden = 24
	nFwdIn  = NT + NA + nState + NDrive + 1
	gamma   = 0.6
)

// Brain is a small neural network that learns, by trial and error, how good
// each (thing, action) is in the current situation, plus a forward model
// that guesses what will happen. Memory is an echo state: a fixed random
// recurrent layer that keeps a fading trace of recent actions and outcomes.
type Brain struct {
	W1  [][]float32 `json:"w1"` // nHidden x nIn
	B1  []float32   `json:"b1"`
	W2  []float32   `json:"w2"`
	B2  float32     `json:"b2"`
	F   [][]float32 `json:"f"` // forward model: NO x nFwdIn
	Mem []float32   `json:"mem"`

	// running prediction error per target/action, for noticing what it has figured out
	Err   [NT][NA]float32 `json:"err"`
	Tried [NT][NA]int     `json:"tried"`
	Novel [NT]float32     `json:"novel"` // 1 = never seen, decays with attention

	LastA   int         `json:"lastA"`
	LastT   int         `json:"lastT"`
	LastOut [NO]float32 `json:"lastOut"`

	memW   [][]float32
	memSrc int64
}

func NewBrain(g Genome) *Brain {
	r := rand.New(rand.NewSource(g.Instinct))
	b := &Brain{B1: make([]float32, nHidden), W2: make([]float32, nHidden), Mem: make([]float32, nMem)}
	b.W1 = make([][]float32, nHidden)
	for i := range b.W1 {
		b.W1[i] = make([]float32, nIn)
		for j := range b.W1[i] {
			b.W1[i][j] = float32(r.NormFloat64()) * 0.25
		}
		b.W2[i] = float32(r.NormFloat64()) * 0.1
	}
	b.F = make([][]float32, NO)
	for i := range b.F {
		b.F[i] = make([]float32, nFwdIn)
	}
	for t := range b.Novel {
		b.Novel[t] = 1
	}
	for t := range b.Err {
		for a := range b.Err[t] {
			b.Err[t][a] = 0.5
		}
	}
	b.initMem(g.Instinct)
	return b
}

// initMem builds the fixed random memory wiring (not saved; rebuilt from seed).
func (b *Brain) initMem(seed int64) {
	r := rand.New(rand.NewSource(seed ^ 0x5eed))
	b.memW = make([][]float32, nMem)
	for i := range b.memW {
		b.memW[i] = make([]float32, nMemIn+nMem)
		for j := range b.memW[i] {
			b.memW[i][j] = float32(r.NormFloat64()) * 0.45
		}
	}
	b.memSrc = seed
	if len(b.Mem) != nMem {
		b.Mem = make([]float32, nMem)
	}
}

// Fix restores the bits of a loaded brain that aren't saved.
func (b *Brain) Fix(g Genome) {
	if b.memW == nil || b.memSrc != g.Instinct {
		b.initMem(g.Instinct)
	}
}

// Remember folds the last action, target and outcome into memory.
func (b *Brain) Remember(a, t int, out [NO]float32) {
	in := make([]float32, nMemIn+nMem)
	in[a] = 1
	in[NA+t] = 1
	copy(in[NA+NT:], out[:])
	copy(in[nMemIn:], b.Mem)
	next := make([]float32, nMem)
	for i := range next {
		var s float32
		for j, v := range in {
			s += b.memW[i][j] * v
		}
		next[i] = 0.5*b.Mem[i] + 0.5*float32(math.Tanh(float64(s)))
	}
	copy(b.Mem, next)
	b.LastA, b.LastT, b.LastOut = a, t, out
}

// Situation is what the plip perceives about one target.
type Situation struct {
	Drives [NDrive]float32
	State  [NT][nState]float32
}

func (b *Brain) input(s *Situation, t, a int) []float32 {
	x := make([]float32, nIn)
	k := copy(x, s.Drives[:])
	x[k+t] = 1
	k += NT
	x[k+a] = 1
	k += NA
	k += copy(x[k:], s.State[t][:])
	x[k+b.LastA] = 1
	x[k+NA+b.LastT] = 1
	copy(x[k+NA+NT:], b.LastOut[:])
	k += nMemIn
	k += copy(x[k:], b.Mem)
	x[k] = 1
	return x
}

// Q is how good the brain thinks doing a to t is right now. It returns the
// hidden activations too, for learning.
func (b *Brain) Q(x []float32) (float32, []float32) {
	h := make([]float32, nHidden)
	q := b.B2
	for i := range h {
		s := b.B1[i]
		for j, v := range x {
			if v != 0 {
				s += b.W1[i][j] * v
			}
		}
		h[i] = float32(math.Tanh(float64(s)))
		q += b.W2[i] * h[i]
	}
	return q, h
}

// Train nudges Q(x) toward target by one step of backprop.
func (b *Brain) Train(x []float32, target, lr float32) {
	q, h := b.Q(x)
	err := clamp(target-q, -2, 2)
	for i := range h {
		g := err * b.W2[i] * (1 - h[i]*h[i])
		b.W2[i] += lr * err * h[i]
		b.B1[i] += lr * g
		for j, v := range x {
			if v != 0 {
				b.W1[i][j] = clamp(b.W1[i][j]+lr*g*v, -5, 5)
			}
		}
	}
	b.B2 += lr * err
}

func (b *Brain) fwdInput(s *Situation, t, a int) []float32 {
	x := make([]float32, nFwdIn)
	x[t] = 1
	x[NT+a] = 1
	k := NT + NA
	k += copy(x[k:], s.State[t][:])
	k += copy(x[k:], s.Drives[:])
	x[k] = 1
	return x
}

// Predict guesses the outcome of doing a to t.
func (b *Brain) Predict(s *Situation, t, a int) (out [NO]float32) {
	x := b.fwdInput(s, t, a)
	for o := range out {
		var v float32
		for j, xv := range x {
			v += b.F[o][j] * xv
		}
		out[o] = v
	}
	return
}

// Surprise compares a guess with what happened, learns from it, and
// returns how surprised the plip is (0 = saw it coming).
func (b *Brain) Surprise(s *Situation, t, a int, got [NO]float32) float32 {
	x := b.fwdInput(s, t, a)
	pred := b.Predict(s, t, a)
	var e float32
	for o := range got {
		d := got[o] - pred[o]
		e += d * d
		for j, xv := range x {
			if xv != 0 {
				b.F[o][j] += 0.15 * d * xv
			}
		}
	}
	e /= NO
	b.Err[t][a] = b.Err[t][a]*0.8 + e*0.2
	b.Tried[t][a]++
	return e
}

// Liking is how keen the brain is to eat colour c when fairly hungry.
func (b *Brain) Liking(s *Situation, c int) float32 {
	ss := *s
	ss.Drives[Hunger] = 0.7
	q, _ := b.Q(b.input(&ss, c, AEat))
	return q
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

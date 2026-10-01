package sim

import (
	"math"
	"math/rand"
)

// The motor net is the part of the brain that drives the legs. Nothing about
// walking is scripted: it maps a step rhythm plus what each foot feels to a
// hip and knee angle per leg, and finds a gait by trial and error. Each trial
// it tries its weights nudged one way, then the same nudge the other way, and
// moves toward whichever carried it further (antithetic weight perturbation,
// a tiny evolution strategy).

const (
	MaxLegs = 4

	mIn     = 10 // sin φ, cos φ, sin 2φ, cos 2φ, foot contact x4, lift, bias
	mOut    = MaxLegs * 2
	mParams = mOut*mIn + 1 // + rhythm speed

	trialLen = 80   // ticks of trying to move per trial
	mSigma   = 0.3  // size of each nudge
	mAlpha   = 0.12 // how far to step toward the better nudge
)

type Motor struct {
	W     [mOut][mIn]float32
	Freq  float32 // step rhythm, before squashing
	Skill float32 // running average speed while trying to move, px/tick
	Tries int     // trial pairs done

	eps         [mParams]float32
	sgn         float32
	acc, rPlus  float32
	n           int
	diffScale   float32
	initialised bool
}

// NewMotor wires a random, uncoordinated motor net: the legs flail at first.
func NewMotor(seed int64) *Motor {
	r := rand.New(rand.NewSource(seed ^ 0x5eed1e95))
	m := &Motor{}
	for o := range m.W {
		for i := range m.W[o] {
			m.W[o][i] = float32(r.NormFloat64()) * 0.5
		}
	}
	m.Freq = float32(r.NormFloat64()) * 0.5
	return m
}

func (m *Motor) param(k int) *float32 {
	if k == mParams-1 {
		return &m.Freq
	}
	return &m.W[k/mIn][k%mIn]
}

func (m *Motor) start(r *rand.Rand) {
	for k := range m.eps {
		m.eps[k] = float32(r.NormFloat64())
	}
	m.sgn, m.acc, m.n = 1, 0, 0
	m.initialised = true
}

func (m *Motor) w(o, i int) float32 { return m.W[o][i] + m.sgn*mSigma*m.eps[o*mIn+i] }

// rate is how far the step rhythm advances per tick.
func (m *Motor) rate() float32 {
	f := m.Freq + m.sgn*mSigma*m.eps[mParams-1]
	return 0.04 + 0.2/(1+float32(math.Exp(-float64(f))))
}

// targets gives (hip, knee) per leg. Hip 0 is straight down, positive swings
// forward; knee 0 is straight, larger bends the shin back.
func (m *Motor) targets(ph float32, contact [MaxLegs]float32, lift float32) (out [MaxLegs][2]float32) {
	var x [mIn]float32
	s1, c1 := math.Sincos(float64(ph))
	s2, c2 := math.Sincos(float64(2 * ph))
	x[0], x[1], x[2], x[3] = float32(s1), float32(c1), float32(s2), float32(c2)
	copy(x[4:8], contact[:])
	x[8], x[9] = lift, 1
	for k := 0; k < MaxLegs; k++ {
		var h, kn float32
		for i, v := range x {
			h += m.w(2*k, i) * v
			kn += m.w(2*k+1, i) * v
		}
		out[k][0] = 1.1 * tanh(h)
		out[k][1] = 0.8 * (1 + tanh(kn))
	}
	return
}

// learn is fed how far the body got toward where it wanted to go this tick.
func (m *Motor) learn(r *rand.Rand, progress float32) {
	if !m.initialised {
		m.start(r)
	}
	m.acc += progress
	m.n++
	if m.n < trialLen {
		return
	}
	R := m.acc / float32(m.n)
	if m.sgn > 0 {
		m.rPlus, m.sgn, m.acc, m.n = R, -1, 0, 0
		return
	}
	d := m.rPlus - R
	ad := float32(math.Abs(float64(d)))
	if m.diffScale == 0 {
		m.diffScale = ad + 1e-3
	}
	m.diffScale = m.diffScale*0.95 + ad*0.05
	step := mAlpha * mSigma * clamp(d/(m.diffScale+1e-3), -3, 3)
	for k := 0; k < mParams; k++ {
		p := m.param(k)
		*p = clamp(*p+step*m.eps[k], -4, 4)
	}
	m.Skill = m.Skill*0.92 + (m.rPlus+R)/2*0.08
	m.Tries++
	m.start(r)
}

func tanh(v float32) float32 { return float32(math.Tanh(float64(v))) }

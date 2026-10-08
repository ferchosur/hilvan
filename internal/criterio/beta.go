package criterio

import "math"

// Mulberry32 is a seeded PRNG so recommendations are reproducible in tests.
// It reproduces the TypeScript implementation bit for bit.
func Mulberry32(seed uint32) func() float64 {
	a := seed
	return func() float64 {
		a += 0x6d2b79f5
		t := a
		t = (t ^ (t >> 15)) * (t | 1)
		t ^= t + (t^(t>>7))*(t|61)
		return float64(t^(t>>14)) / 4294967296
	}
}

func normal(rng func() float64) float64 {
	// Box–Muller
	u := math.Max(rng(), 1e-12)
	v := rng()
	return math.Sqrt(-2*math.Log(u)) * math.Cos(2*math.Pi*v)
}

// SampleGamma draws Gamma(k, 1) with Marsaglia–Tsang; valid for any k > 0.
func SampleGamma(k float64, rng func() float64) float64 {
	if k < 1 {
		g := SampleGamma(k+1, rng)
		return g * math.Pow(rng(), 1/k)
	}
	d := k - 1.0/3
	c := 1 / math.Sqrt(9*d)
	for {
		var x, v float64
		for {
			x = normal(rng)
			v = 1 + c*x
			if v > 0 {
				break
			}
		}
		v = v * v * v
		u := rng()
		if u < 1-0.0331*math.Pow(x, 4) || math.Log(u) < 0.5*x*x+d*(1-v+math.Log(v)) {
			return d * v
		}
	}
}

// SampleBeta draws Beta(a, b).
func SampleBeta(a, b float64, rng func() float64) float64 {
	x := SampleGamma(a, rng)
	y := SampleGamma(b, rng)
	return x / (x + y)
}

// BetaArm holds success/failure counts for one (profile, task type), with a Beta(1,1) prior.
type BetaArm struct {
	A float64 `json:"a"`
	B float64 `json:"b"`
}

// Prior is the uniform Beta(1, 1).
var Prior = BetaArm{A: 1, B: 1}

func (arm BetaArm) update(success bool, weight float64) BetaArm {
	if success {
		return BetaArm{A: arm.A + weight, B: arm.B}
	}
	return BetaArm{A: arm.A, B: arm.B + weight}
}

// Mean is the expected success rate.
func (arm BetaArm) Mean() float64 { return arm.A / (arm.A + arm.B) }

// Observations is the number of recorded outcomes.
func (arm BetaArm) Observations() float64 { return arm.A + arm.B - Prior.A - Prior.B }

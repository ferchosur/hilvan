package criterio

import "math"

// Outcome is a past probabilistic prediction and what actually happened.
type Outcome struct {
	P float64 `json:"p"`
	Y int     `json:"y"`
}

// Brier is the mean squared error of probabilities; 0 is perfect, 0.25 a coin flip.
// ok is false when there are no outcomes.
func Brier(outcomes []Outcome) (score float64, ok bool) {
	if len(outcomes) == 0 {
		return 0, false
	}
	s := 0.0
	for _, o := range outcomes {
		s += math.Pow(o.P-float64(o.Y), 2)
	}
	return s / float64(len(outcomes)), true
}

// ReliabilityBin is how often predictions in one confidence band were right.
type ReliabilityBin struct {
	From     float64 `json:"from"`
	To       float64 `json:"to"`
	Count    int     `json:"count"`
	MeanP    float64 `json:"meanP"`
	Observed float64 `json:"observed"`
}

// Reliability builds a reliability curve with the given number of bins.
func Reliability(outcomes []Outcome, bins int) []ReliabilityBin {
	out := make([]ReliabilityBin, 0, bins)
	for i := 0; i < bins; i++ {
		from := float64(i) / float64(bins)
		to := float64(i+1) / float64(bins)
		var sumP, sumY float64
		count := 0
		for _, o := range outcomes {
			if o.P >= from && ((i == bins-1 && o.P <= to) || (i != bins-1 && o.P < to)) {
				count++
				sumP += o.P
				sumY += float64(o.Y)
			}
		}
		bin := ReliabilityBin{From: from, To: to, Count: count}
		if count > 0 {
			bin.MeanP = sumP / float64(count)
			bin.Observed = sumY / float64(count)
		}
		out = append(out, bin)
	}
	return out
}

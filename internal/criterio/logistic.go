package criterio

import (
	"bytes"
	"encoding/json"
	"math"
	"sort"
	"strconv"
	"strings"

	"hilvan/internal/jsfmt"
)

// readable labels a feature for explanations; structural features are not shown.
func readable(name string) (string, bool) {
	switch {
	case strings.HasPrefix(name, "w:"), strings.HasPrefix(name, "r:"):
		return `"` + name[2:] + `"`, true
	case strings.HasPrefix(name, "b:"):
		return `"` + strings.Replace(name[2:], "_", " ", 1) + `"`, true
	case strings.HasPrefix(name, "diff."), strings.HasPrefix(name, "state:"), name == "denials":
		return name, true
	}
	return "", false
}

// SoftmaxState is the serialised model: only hashed buckets with non-zero weights.
type SoftmaxState struct {
	Classes int `json:"classes"`
	// Weights maps bucket → weight per class. JSON keys are emitted in ascending numeric
	// order, which is how JavaScript enumerates integer-like object keys.
	Weights Weights `json:"weights"`
	Updates int     `json:"updates"`
}

// Weights maps a bucket to its per-class weights.
type Weights map[uint32][]float64

// MarshalJSON writes buckets in ascending numeric order.
func (w Weights) MarshalJSON() ([]byte, error) {
	keys := make([]uint32, 0, len(w))
	for k := range w {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(`"` + strconv.FormatUint(uint64(k), 10) + `":[`)
		for j, x := range w[k] {
			if j > 0 {
				b.WriteByte(',')
			}
			b.WriteString(jsfmt.Number(x))
		}
		b.WriteByte(']')
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// UnmarshalJSON accepts any decimal keys; Validate rejects the rest.
func (w *Weights) UnmarshalJSON(data []byte) error {
	var raw map[string][]float64
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*w = Weights{}
	for k, v := range raw {
		n, err := strconv.ParseUint(k, 10, 32)
		if err != nil {
			return &json.UnsupportedValueError{Str: "weights key " + strconv.Quote(k)}
		}
		(*w)[uint32(n)] = v
	}
	return nil
}

// OnlineSoftmax is online multinomial logistic regression over hashed sparse features
// (SGD + L2). Binary questions use 2 classes.
type OnlineSoftmax struct {
	classes      int
	w            map[uint32][]float64
	t            int
	lr, l2, clip float64
}

// NewOnlineSoftmax returns an empty model.
func NewOnlineSoftmax(classes int) *OnlineSoftmax {
	return &OnlineSoftmax{classes: classes, w: map[uint32][]float64{}, lr: 0.5, l2: 1e-4, clip: 4}
}

// Updates is the number of training steps.
func (m *OnlineSoftmax) Updates() int { return m.t }

func (m *OnlineSoftmax) logits(feats []Feature) []float64 {
	z := make([]float64, m.classes)
	for _, f := range feats {
		row, ok := m.w[Bucket(f.Name)]
		if !ok {
			continue
		}
		for c := 0; c < m.classes; c++ {
			z[c] += row[c] * f.Value
		}
	}
	return z
}

// Predict returns class probabilities.
func (m *OnlineSoftmax) Predict(feats []Feature) []float64 { return Softmax(m.logits(feats)) }

// Explain returns the signals that most support class c, as readable reasons.
func (m *OnlineSoftmax) Explain(feats []Feature, c, top int) []Reason {
	out := []Reason{}
	for _, f := range feats {
		if f.Name == "bias" {
			continue
		}
		row, ok := m.w[Bucket(f.Name)]
		if !ok {
			continue
		}
		others := 0.0
		for k := 0; k < m.classes; k++ {
			if k != c {
				others += row[k]
			}
		}
		contrib := (row[c] - others/math.Max(1, float64(m.classes-1))) * f.Value
		if label, ok := readable(f.Name); ok && math.Abs(contrib) > 0.05 {
			out = append(out, Reason{Signal: "aprendido: " + label, Weight: contrib})
		}
	}
	out = sortByWeightDesc(out)
	if len(out) > top {
		out = out[:top]
	}
	return out
}

// Update takes one SGD step towards label y; weight scales the step (corrections count more).
func (m *OnlineSoftmax) Update(feats []Feature, y int, weight float64) {
	m.t++
	p := m.Predict(feats)
	eta := m.lr / math.Sqrt(float64(m.t)) * weight
	for _, f := range feats {
		b := Bucket(f.Name)
		row, ok := m.w[b]
		if !ok {
			row = make([]float64, m.classes)
			m.w[b] = row
		}
		for c := 0; c < m.classes; c++ {
			target := 0.0
			if c == y {
				target = 1
			}
			grad := (p[c]-target)*f.Value + m.l2*row[c]
			row[c] = math.Max(-m.clip, math.Min(m.clip, row[c]-eta*grad))
		}
	}
}

// State serialises the model, rounding weights to 5 decimals and dropping near-zero rows.
func (m *OnlineSoftmax) State() SoftmaxState {
	weights := Weights{}
	for b, row := range m.w {
		keep := false
		for _, x := range row {
			if math.Abs(x) > 1e-6 {
				keep = true
				break
			}
		}
		if keep {
			r := make([]float64, len(row))
			for i, x := range row {
				r[i] = jsfmt.Round(x*1e5) / 1e5
			}
			weights[b] = r
		}
	}
	return SoftmaxState{Classes: m.classes, Weights: weights, Updates: m.t}
}

// SoftmaxFromState restores a model; rows of the wrong length are ignored.
func SoftmaxFromState(s SoftmaxState) *OnlineSoftmax {
	m := NewOnlineSoftmax(s.Classes)
	m.t = s.Updates
	for b, row := range s.Weights {
		if len(row) == s.Classes {
			m.w[b] = append([]float64(nil), row...)
		}
	}
	return m
}

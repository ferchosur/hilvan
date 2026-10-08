package criterio

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/ferchosur/hilvan/internal/jsfmt"
)

// ordered is a string-keyed map that keeps insertion order, like a JavaScript Map or an object
// with non-integer keys. Criterio's state relies on that order (stats listing, JSON output).
type ordered[V any] struct {
	keys []string
	m    map[string]V
}

func newOrdered[V any]() *ordered[V] { return &ordered[V]{m: map[string]V{}} }

func (o *ordered[V]) get(k string) (V, bool) {
	v, ok := o.m[k]
	return v, ok
}

func (o *ordered[V]) set(k string, v V) {
	if _, ok := o.m[k]; !ok {
		o.keys = append(o.keys, k)
	}
	o.m[k] = v
}

func (o *ordered[V]) len() int { return len(o.keys) }

func (o *ordered[V]) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			b.WriteByte(',')
		}
		v, err := json.Marshal(o.m[k])
		if err != nil {
			return nil, err
		}
		b.WriteString(jsfmt.String(k))
		b.WriteByte(':')
		b.Write(v)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// UnmarshalJSON keeps the key order of the document. A repeated key keeps its first position
// and its last value, as JSON.parse does.
func (o *ordered[V]) UnmarshalJSON(data []byte) error {
	*o = ordered[V]{m: map[string]V{}}
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	if tok == nil {
		return nil
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return fmt.Errorf("expected an object")
	}
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return err
		}
		k, _ := kt.(string)
		var v V
		if err := dec.Decode(&v); err != nil {
			return err
		}
		o.set(k, v)
	}
	_, err = dec.Token()
	return err
}

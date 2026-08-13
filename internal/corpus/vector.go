package corpus

import (
	"fmt"
	"strconv"
	"strings"
)

// Encode renders a vector as the literal CockroachDB accepts for VECTOR, e.g.
// "[0.1,0.2,0.3]".
//
// The width is checked here rather than left to the database. CockroachDB does
// reject a wrong-width vector ("expected 1024 dimensions, not 3"), but that error
// arrives from the driver at execution time with no indication of which record
// produced it; catching it in Go keeps the caller's context attached.
func Encode(v []float32) (string, error) {
	if len(v) != Dim {
		return "", fmt.Errorf("vector has %d dimensions, want %d", len(v), Dim)
	}
	var b strings.Builder
	b.Grow(len(v) * 12)
	b.WriteByte('[')
	for i, f := range v {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(float64(f), 'g', -1, 32))
	}
	b.WriteByte(']')
	return b.String(), nil
}

// Decode parses a CockroachDB VECTOR literal back into a slice.
func Decode(s string) ([]float32, error) {
	s = strings.TrimSpace(s)
	if len(s) < 2 || s[0] != '[' || s[len(s)-1] != ']' {
		return nil, fmt.Errorf("not a vector literal: %q", s)
	}
	body := s[1 : len(s)-1]
	if body == "" {
		return nil, nil
	}
	parts := strings.Split(body, ",")
	out := make([]float32, 0, len(parts))
	for _, p := range parts {
		f, err := strconv.ParseFloat(strings.TrimSpace(p), 32)
		if err != nil {
			return nil, fmt.Errorf("parse vector element %q: %w", p, err)
		}
		out = append(out, float32(f))
	}
	return out, nil
}

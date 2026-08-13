package corpus

import (
	"hash/fnv"
	"math"
	"strings"
	"unicode"
)

// HashEmbed is a PLACEHOLDER. It is not an embedding model and must never be
// presented as one.
//
// # What it actually does
//
// It is the hashing trick: tokenize on non-letters, fold to lower case, hash each
// token to a dimension, accumulate, then L2-normalize. Two documents land near
// each other exactly when they share literal tokens. That is LEXICAL overlap, not
// semantic similarity -- it cannot relate "raft leader election" to "consensus
// timeout", which is precisely the relation a real embedding model exists to
// capture and the entire reason the corpus needs one.
//
// # Why it exists anyway
//
// Phase 2 proves one narrow thing: that CockroachDB stores vectors, builds a
// vector index over them, and returns correctly ordered nearest neighbours. That
// proof needs vectors with *some* deterministic structure, or the ANN assertions
// degrade to "the query returned k rows" while proving nothing about ordering.
// Lexical locality is enough structure to make retrieval assertions meaningful and
// is reproducible without a network call, a model, or credentials.
//
// # What it is not evidence of
//
// Nothing about retrieval quality. "Does ANN retrieval surface genuinely relevant
// history?" is a later question that only a real model can answer, and it must not
// be claimed on the strength of this function.
//
// # Disposal
//
// Phase 4 replaces this with Amazon Titan Text Embeddings v2, which is why Dim is
// already 1024 -- Titan v2's default and maximum width -- so the column, the index
// and the query path do not change when the real embedder arrives. Delete this
// file at that point rather than leaving two embedders in the tree.
func HashEmbed(text string) []float32 {
	v := make([]float32, Dim)

	for _, tok := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		h := fnv.New32a()
		_, _ = h.Write([]byte(tok))
		sum := h.Sum32()
		idx := int(sum % uint32(Dim))
		// Sign from a high bit so distinct tokens can cancel rather than only
		// accumulate, which keeps unrelated documents from drifting toward a
		// common all-positive direction.
		if sum&0x80000000 != 0 {
			v[idx] -= 1
		} else {
			v[idx] += 1
		}
	}

	var norm float64
	for _, f := range v {
		norm += float64(f) * float64(f)
	}
	norm = math.Sqrt(norm)
	if norm == 0 {
		// An empty or token-free document. Return a fixed unit vector rather than
		// zeros: a zero vector has undefined cosine distance to everything.
		v[0] = 1
		return v
	}
	for i := range v {
		v[i] = float32(float64(v[i]) / norm)
	}
	return v
}

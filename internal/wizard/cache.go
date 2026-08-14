package wizard

import "sync"

// queryCache is a tiny LRU of genuine Bedrock query vectors, keyed by exact query
// text.
//
// # What it is and is not
//
// It is a latency and cost optimisation for a demo that asks the same two questions
// repeatedly. It is NOT a fallback. Nothing synthetic ever enters it: the only writer
// is the code path that has just received a real 1024-float vector from Titan. A miss
// with no credentials fails loudly, exactly as a cold start would — the cache can
// make the demo faster, never make a broken deployment look healthy.
//
// Keyed on exact text because the vector is a function of exact text. Normalising the
// key (trimming, lowercasing) would serve a different query's vector under this
// query's name, and every distance downstream would be quietly wrong.
//
// In memory only, dropped on restart, and it never touches persisted corpus state.
type queryCache struct {
	mu    sync.Mutex
	max   int
	order []string // least-recently-used first
	vecs  map[string][]float32
}

func newQueryCache(max int) *queryCache {
	if max <= 0 {
		max = 1
	}
	return &queryCache{max: max, vecs: make(map[string][]float32, max)}
}

func (c *queryCache) get(q string) ([]float32, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.vecs[q]
	if !ok {
		return nil, false
	}
	c.touch(q)
	// Copied out: callers hand the slice to corpus.Encode and must not be able to
	// mutate a cached vector through it.
	out := make([]float32, len(v))
	copy(out, v)
	return out, true
}

func (c *queryCache) put(q string, v []float32) {
	if len(v) == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.vecs[q]; !exists && len(c.order) >= c.max {
		oldest := c.order[0]
		c.order = c.order[1:]
		delete(c.vecs, oldest)
	}
	stored := make([]float32, len(v))
	copy(stored, v)
	c.vecs[q] = stored
	c.touch(q)
}

// touch moves q to the most-recently-used end. Caller holds the lock.
func (c *queryCache) touch(q string) {
	for i, k := range c.order {
		if k == q {
			c.order = append(c.order[:i], c.order[i+1:]...)
			break
		}
	}
	c.order = append(c.order, q)
}

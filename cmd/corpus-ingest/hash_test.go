package main

import "testing"

// TestGoldenContentHash pins the corpus content-hash specification.
//
// The hash is the content address of a corpus row, so its stability is a data
// contract, not an implementation detail: change the field set, the field order,
// the JSON tags, or the label sorting, and every one of the ~7,239 rows silently
// acquires a different identity while the ingest still reports success. This test
// is the tripwire. If it fails, the hash spec changed — decide deliberately, then
// update the constant.
func TestGoldenContentHash(t *testing.T) {
	closed := "2022-04-20T00:00:00Z"
	iss := SnapshotIssue{
		Number:    13514,
		Title:     "etcd v3.5 data inconsistency",
		Body:      "consistent index not saved atomically",
		State:     "closed",
		URL:       "https://github.com/etcd-io/etcd/issues/13514",
		CreatedAt: "2021-11-24T00:00:00Z",
		UpdatedAt: "2022-04-20T00:00:00Z",
		ClosedAt:  &closed,
		Labels:    []string{"type/bug", "area/storage"},
	}

	const golden = "50493254679b6e0fbe3e77f790714285116a1229b7662fdab5850e1696dd29c1"
	got := contentHash("etcd-io/etcd", iss)
	if got != golden {
		t.Fatalf("content hash changed\n  got:    %s\n  golden: %s\n"+
			"If this was deliberate, every existing corpus row's hash is now stale.", got, golden)
	}
}

// Label order is upstream's business, not ours. GitHub does not promise a stable
// order, so an unsorted hash would make the same issue appear to change.
func TestContentHashIgnoresLabelOrder(t *testing.T) {
	base := SnapshotIssue{Number: 1, Title: "t", Body: "b", State: "open",
		Labels: []string{"alpha", "beta", "gamma"}}
	shuffled := base
	shuffled.Labels = []string{"gamma", "alpha", "beta"}

	if contentHash("r", base) != contentHash("r", shuffled) {
		t.Fatal("hash depends on label ordering; it must not")
	}
}

// State and closed_at participate deliberately: a reopen is exactly the upstream
// change the hash exists to expose.
func TestContentHashTracksStateChange(t *testing.T) {
	closed := "2022-04-20T00:00:00Z"
	a := SnapshotIssue{Number: 1, Title: "t", Body: "b", State: "closed", ClosedAt: &closed}
	b := a
	b.State = "open"
	b.ClosedAt = nil

	if contentHash("r", a) == contentHash("r", b) {
		t.Fatal("hash is blind to state/closed_at; reopening would go undetected")
	}
}

// A nil label slice and an empty one are the same fact about an issue.
func TestContentHashNilAndEmptyLabelsAgree(t *testing.T) {
	a := SnapshotIssue{Number: 1, Title: "t", State: "open", Labels: nil}
	b := SnapshotIssue{Number: 1, Title: "t", State: "open", Labels: []string{}}

	if contentHash("r", a) != contentHash("r", b) {
		t.Fatal("nil and empty label sets hash differently")
	}
}

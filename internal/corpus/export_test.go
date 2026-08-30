package corpus

// SQLSearchForTest exposes the production ANN statement to the external test
// package.
//
// This exists because of a real regression that a hand-written test query failed
// to catch: TestC05 asserted index usage against a query string typed into the
// test, so when an "AND embedding IS NOT NULL" predicate was added to the actual
// sqlSearch it silently downgraded the plan from a vector search to a full scan
// while the test stayed green. Tests must EXPLAIN the statement that really runs,
// not a lookalike.
var SQLSearchForTest = SearchSQL()

// Package proof makes the phase's measured evidence available to the running binary.
//
// The transcripts are embedded rather than read from disk, which has one property worth
// the file: /proof structurally cannot display a transcript that is not committed to the
// repository. There is no path where the page renders something a reviewer could not
// find in git, and no deployment where the evidence is missing because a COPY was
// forgotten.
//
// These are outputs, not inputs. Nothing here interprets them; they were produced by
// proof/harness/cells_crdb.sh and proof/act6_tier_probe.sql against real clusters, and
// the page shows them verbatim.
package proof

import (
	_ "embed"
)

// Isolation is proof/isolation.log: the three-cell isolation experiment, cells 1 and 2
// on local CockroachDB v26.2.0 and cell 3 on great-goat v26.2.5.
//
//go:embed isolation.log
var Isolation string

// Act6Tier is proof/act6_tier_probe.log: the probe that settled which Act 6 climax is
// available, run on both clusters.
//
//go:embed act6_tier_probe.log
var Act6Tier string

I reviewed the implementation spec. **Do not send it to the coding agent yet.** It is much better, but there are still a few concrete contradictions that will cause implementation/replay problems.

### The important blockers

1. **Replay is not actually idempotent for intents.**

The spec itself admits:

> `IntentOnPromoted` creates duplicate intent — replay must be idempotent at pipeline level. 

Those two statements conflict. If the pipeline calls `IntentOnPromoted` again, it creates another intent unless the existing kernel/schema has a uniqueness mechanism. Therefore the acceptance criterion:

> "Replay produces zero new rows"

cannot currently be true. 

**Do not modify the kernel just for this.** For MVP, separate:

* deterministic evidence/belief replay
* intent creation as an explicit post-promotion demo action

rather than pretending the entire workflow is idempotent.

---

2. **Track 2 cannot use the same fixture directory as currently specified.**

The CLI says Track 2 runs:

```text
--fixtures internal/derive/testdata/etcd_real
```

But that directory contains Track 1 and Track 2 records. The pipeline processes `.json` files from the directory and applies the scenario ID supplied to the run. 

So Track 2 would ingest:

* GHSA
* v3.5.27
* v3.5.28
* v3.5.0
* postmortem

under the Track 2 scenario unless there is filtering that the spec hasn't established.

**Fix:** use separate fixture directories:

```text
etcd_real/
  track1/
    ghsa_advisory.json
    release_v3527.json
    release_v3528.json

  track2/
    release_v350.json
    postmortem_v35.json
```

This is cleaner and actually makes the two-track story much easier to demonstrate.

---

3. **`maintainer_review.json` contradicts the fixture rules.**

The spec says:

> `maintainer_review.json # maintainer_comment (raw reference, not processed by pipeline)` 

But it also says:

> "Pipeline reads directory, sorts `.json` files, processes each." 

And:

> "Each `.json` file ... must be registered in `sourceTypeMap`." 

Therefore this raw reference file **cannot sit in the ingested directory** unless the pipeline is changed to ignore it.

Move raw/reference material outside the ingestion directory, e.g.:

```text
etcd_real/
  track1/
    *.json

  track2/
    *.json

  raw/
    maintainer_review.json

  manifest.json
  ATTRIBUTION.md
```

Or simply don't include the unused file.

---

4. **The evidence count is wrong.**

The Track 1 design has exactly three processed records:

* GHSA
* v3.5.27
* v3.5.28

and each produces one evidence attachment. The transcript correctly says:

> `evidence_attached: 3` 

But the demo beats say:

> "3 beliefs entered, 6 evidence records ingested" 

That should be **3 evidence records**, unless there is another intended evidence path that isn't documented.

---

5. **The "5 new fixture filenames" is wrong/ambiguous.**

The implementation section says:

> `sourceTypeMap` includes all 5 new fixture filenames 

But only two source types are new, and the proposed actual pipeline records are:

* GHSA
* v3.5.27
* v3.5.28
* v3.5.0
* postmortem

That's five files, but only **two require new source-type mappings**.

The spec should distinguish:

```text
new source types:
  github_advisory
  postmortem

fixture files:
  5 total
```

---

## One thing I would NOT change

The overall architecture is now good.

The most important conceptual correction is preserved:

```text
Track 1:
real external evidence
    ↓
belief/debt
    ↓
operator review
    ↓
promotion
    ↓
intent

Track 2:
postulated historical decision
    ↓
live intent
    ↓
real postmortem evidence
    ↓
falsify
    ↓
RetractCascade
    ↓
cancelled intent
```

That is a much more honest demo than the original Gemini design.

The spec also correctly keeps the kernel frozen and limits the new work to the normalize/derive/mapping/pipeline layers plus an operator mechanism. 

## My verdict

**Status: REWORK REQUIRED — but only a small spec cleanup.**

Do **not** ask for another research pass. Do **not** ask Fable to redo the whole adversarial review.

Give the **same coding agent** a short correction task:

> Fix the four inconsistencies above, resolve the replay/intent semantics explicitly, separate Track 1 and Track 2 fixtures, remove the unprocessed JSON ambiguity, correct the evidence counts, then update the implementation spec. Do not write production code.

After that, **we review the revised spec once**, and then I would green-light implementation.

This is still comfortably within the MVP path; I would not let the agent turn these corrections into another architectural redesign.

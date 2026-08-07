# WAVE2 ADVERSARIAL REVIEW

**Date:** 2026-08-08
**Scope:** `internal/derive` only — `derive.go`, `types.go`, `derive_test.go`, `testdata/`, and the implementation receipt `WAVE2_IMPLEMENTATION.md`.
**Authority used:** `AGENTS.md`, `IMPLEMENTATION_CONTRACT.md`, `DATA_PIPELINE_SPEC.md`, `WAVE2_IMPLEMENTATION_PLAN.md`.
**Method:** four adversarial passes (distributed systems, data engineering, adversarial systems, spec conformance) with cross-pass synthesis and empirical probes. All probe tests were temporary and removed.

---

## Verdict: REWORK REQUIRED

Wave 2 derive is pure, deterministic, correctly scoped, and passes every receipt. However it carries **two P1 trust-boundary defects** (fabricated "confirmed by maintainer" claims; "fix available" from unmerged PRs) and **one P1 unreachable-branch defect** (the release "confirmed" path cannot fire in the real pipeline, making Rule 2's classification deviate from the plan). Root cause is uniform: the rule engine decides on fields that differ from what the pipeline actually emits.

---

## Receipts

All green, re-run after probe removal:

```
go build ./...                  → exit 0
go vet ./...                    → exit 0
go test ./internal/derive/...   → 10/10 PASS
go test ./internal/normalize/... → PASS (Wave 1 intact, frozen untouched)
go test ./internal/kernel/...   → PASS
```

---

## Pass Summaries

### Pass 1 — Distributed Systems: PASS

- `Derive` is a pure function: no clock reads, no randomness, no I/O, no global mutable state. `TestDerive_Determinism` (100 iterations) produces identical beliefs.
- Idempotency: derive is stateless; Wave 3 persistence must key off content hashes, not a run counter — no hidden state introduced here.
- No architectural drift: no kernel/DB imports, no schema touches, no orchestration, kernel files untouched.
- Defaults handled: unknown `SourceType` → `nil`; empty subject → `"unknown product"`.

### Pass 2 — Data Engineering: FAIL

- Provenance chain preserved: claims carry `SupportingEvidence`; contradictions carry `Contradicts`. Good.
- Classification fidelity breaks (findings F1, F2, F3): derived metadata (`is_maintainer`, `merged`, release `keywords`) is discarded at the boundary where it matters.

### Pass 3 — Adversarial: FAIL

- Trust boundary bypass: non-maintainer comments produce claims literally stating "confirmed by maintainer" (F1, empirically proven).
- Unmerged PRs (`merged:false`) produce "fix available" (F2, empirically proven).
- Malformed/empty `DomainPayload` is swallowed silently, weakening classification instead of failing loudly (F4).

### Pass 4 — Architecture conformance: FAIL

- Rule 1 (KEV): conforms. Rule 4 (contradiction): conforms, except the plan's bare "regression" trigger is not implemented (minor).
- Rule 2 (release): **classification deviates** from plan (plan: always `Accommodated`; impl: only when payload has fix/test keywords — which in reality never happens, see F3).
- Rule 2 (PR): **deviates** from plan (`if merged and CVE references` — impl checks only the word "fix").

---

## Critical Findings

### F1 — Non-maintainer input fabricates "confirmed by maintainer" claims [P1]

**Location:** `internal/derive/derive.go` `deriveFromMaintainerComment` (lines 101–143).

`normalize` already embeds `"is_maintainer": <bool>` in the `maintainer_comment` payload (`internal/normalize/normalize.go:269`). Derive never reads it: any comment whose body contains fix keywords yields

```
claim: "fix for etcd-io/etcd confirmed by maintainer"
classification: accommodated
```

**Empirical probe** (since removed): author `"mallory-not-a-maintainer"`, body `"Fixed in v3.5.15. …"` → payload carries `"is_maintainer": false` → output claim still **"confirmed by maintainer,"** classification `accommodated`.

**Impact:** the ledger's core claim is "agents act only on beliefs that are true." This assert false statement is produced deterministically from data that is available at the next function call. Wave 3 wires such beliefs into debt retirement and after-retirement gates. Public GitHub users can plant these without any real confirmation.

**Required change (contained):** require `payload["is_maintainer"] == true` before emitting the maintainer‑confirmation claim. When `false`, either drop the comment (return `nil`) or emit a weaker claim (e.g. `"fix reported for X"`, class `derived`). The data to gate on already exists — no normalize change needed.

---

### F2 — Unmerged PRs emit "fix available" [P1, conformance]

**Location:** `internal/derive/derive.go` `deriveFromGitHubPR` (lines 160–177).

The plan says (`WAVE2_IMPLEMENTATION_PLAN.md §5`):
> `github_pr` — Rule 3: Fix release (**if merged and CVE references**)

Normalize already emits `"merged": <bool>` (normalize.go:191). The implementation instead returns `Accommodated` whenever the parsed Title contains "fix" (case-insensitive). No merged check, no CVE reference check.

**Empirical probe:** PR title `"Fix multipart MIME parsing vulnerability"`, `state: closed`, `merged: false` → output `"fix available for etcd-io/etcd"`, class `accommodated`.

**Impact:** Wave 3 will retire debt and allow agent actions on "fix available" beliefs derived from a PR that never landed. Reads "the database decides", but the discard here too early. Either require a `merged` — or upgrade the rule faithfully according to plan and add a test with
`merged:false` → `nil`/Derived.

---

### F3 — Rule 2 "confirmed" branch is unreachable; real classification ends up Derived, contrary to plan [P1]

**Location:** `internal/derive/derive.go` `deriveFromRelease` (68–99) + `extractKeywordsFromPayload` (179–206); `internal/normalize` `releaseMetadata` (normalize.go:276–283).

The plan §5 (Rule 2) says releases → `Accommodated`; §5 enhancement admits a "confirmed by maintainer" note if the payload says `keywords`.

**Facts:**

1. `normalizeRelease` emits only `[]{version,version_normalized,tag,commit_sha,prerelease,assets}` — there is no `keywords` neither parse. The provided fixture `testdata/release_confirmed.json` has `"keywords": ["fixed","tested","confirmed"]` but `json.Unmarshal` ignores it early, drop occurred.
2. Therefore `extractKeywordsFromPayload` returns `nil` for any real release, `confirmed` is always `false`, classification is always **Derived** — never the planned `Accommodated`.
3. `TestDerive_ReleaseConfirmedByMaintainer` passes only because it hand‑constructs a `DomainPayload` that normalizer never produces; it tests a phantom path.

**Empirical probe:** `release_confirmed.json` fixture→`normalize` → `derive` printed `"fix available for etcd v3.5.15"` … then classified `derived`; payload keys are exactly `{assets,commit_sha,prerelease,tag,version,version_normalized}` — no keywords.

**Recommendations (pick one):**
- **Simplest, spec-conformant:** unconditionally classify releases as `Accommodated` (per plan), delete the unreachable keyword branch, and update the fixture with a `TestRelease*` + payload-key assertion proving that `normalize→derive` path.
- Larger‑goals fix requires extending `normalizeRelease` to emit keywords — impossible since Wave 1 is **frozen** (no change to normalize allowed).

Repair without waiting: choose option one; it removes dead code and restores plan conformance.

---

### F4 — Malformed/absent DomainPayload fails silently [P2]

`extractKeywordsFromPayload` (derive.go:179–206) returns `nil` on JSON error, missing key, or wrong type — so a corrupted Domain payload silently downgrades classification (no fail-loud behavior per contract "Fail on invalid evidence"). Whether the payload `nil` case is legitimately "no keywords" or corrupted cannot be distinguished.

**Recommendation:** make malformed-payload handling explicit: return an error (or a typed failure) from a payload-parse helper instead of silently returning `nil`, so derive can fail loudly per contract. Keep the change deterministic and contained — do not add DB or kernel dependencies.

---

## Minor Notes

- Rule 4 plan trigger includes bare word "regression"; impl matches only `(reproduces|still vulnerable)` — big `"no regression"` is handled, but "regression" as positive finding is missed. Triply marginal: contradictions are enumerated in the spec's dictionary. Minor.
- `vulnPattern` is unanchored (`vulnerable to`, so `assertion` from KEV will match; fine but not reused in issue/PR claims).
- ClaimSubject "unknown product" fallback — good.

---

## Spec Traceability

| Plan/output | Implementation | Verdict |
|---|---|---|
| Rule 1: KEV → derive belief, claim `{subject} is vulnerable to {cve}` (list ok) | ✓ | PASS |
| Rule 2: release → accommodated | “Derived” whenever unconfirmed when unconfirmed in real flow | **FAIL (F3)** |
| Rule 2b: PR → `if merged and CVE references` | only checks the word "fix" | **FAIL (F2)** |
| Rule 3: maintainer confirmation | gated on keywords; never `is_maintainer` | **FAIL (F1)** |
| Rule 4: contradictory evidence → contradiction | ✓ `reproduces`/`still vulnerable` | PASS (minor trigger gap) |
| Attribute: deterministic, domain-agnostic | ✓ | PASS |
| Fail loudly on invalid evidence (prompt requirement) | malformed payload silently swallowed | PARTIAL (P2/F4) |

---

## Conclusion

Build, vet, test: green. Determinism/purity/scope: excellent. The trigger-level trust defects: nothing conceptually broken in Architecture — the rules just ignore metadata that the pipeline leaders already deliver (`is_maintainer`, `merged`), while the keyword-based "confirmed" path that would populate it does not exist in the real flow.

**Recommended minimal set for the fix cycle:**

1. `deriveFromMaintainerComment` — gate on payload `is_maintainer` (remove fabricated trust).
2. `deriveFromGitHubPR` — gate on payload `merged` (or drop outright) + test unmerged case.
3. `deriveFromRelease` — classify `Accommodated` per plan; delete unreachable keyword branch; test with *normalize's actual output*.
4. Make malformed-payload handling explicit rather than nil-silent.

Re-review expected after the fix cycle. — End of review —
# WAVE2_REWORK.md

**Status:** Complete.
**Scope:** Wave 2 rework per `WAVE2_ADVERSARIAL_REVIEW.md` and `wave2_rework_prompt.md`.
**Frozen:** Wave 1 (`internal/normalize`) is frozen. Wave 2 (`internal/derive`) is now frozen.

---

## 1. Findings Resolved

### F1 — Non-maintainer trust boundary [P1]

**Problem:** `deriveFromMaintainerComment` emitted "confirmed by maintainer" for any comment whose body had fix keywords, ignoring the `is_maintainer` field that normalize already embeds in `DomainPayload`.

**Fix:** Added `extractBoolFromPayload` helper. `deriveFromMaintainerComment` now extracts `is_maintainer` from the payload and returns `nil` when `false`. Contradictions bypass this gate (a valid contradiction is valid regardless of who reports it).

**Test:** `TestDerive_NonMaintainerComment` — author `"mallory-not-a-maintainer"`, body has `"Fixed"`, `is_maintainer: false` → returns `nil`.

### F2 — Unmerged PR trust boundary [P1]

**Problem:** `deriveFromGitHubPR` emitted "fix available" whenever the title contained "fix", ignoring the `merged` field that normalize already embeds in `DomainPayload`.

**Fix:** `deriveFromGitHubPR` now extracts `merged` from the payload and returns `nil` when `false`.

**Tests:** `TestDerive_UnmergedPR` — `merged: false` → `nil`. `TestDerive_MergedPR` — `merged: true` → `Accommodated`.

### F3 — Release "confirmed" path unreachable [P1]

**Problem:** The keyword-based `confirmed` branch in `deriveFromRelease` could never fire because normalize never emits `"keywords"` in its release payload. Classification always fell through to `Derived`, contrary to plan §5 which says releases → `Accommodated`.

**Fix:** Removed the unreachable keyword branch entirely. Releases are now unconditionally classified `Accommodated` per plan §5.

**Test:** `TestDerive_FixRelease` — release without keywords → `Accommodated`. `TestDerive_ReleaseWithoutKeyword` — assertion without "release" keyword → `nil`.

### F4 — Malformed payload fails silently [P2]

**Problem:** `extractKeywordsFromPayload` returned `nil` on JSON errors, making it impossible to distinguish "no keywords" from corrupted payload.

**Fix:** Added `extractBoolFromPayload` with explicit `(bool, bool)` return — second value is `false` on malformed payload, missing key, or wrong type. All callers check the second return value explicitly. Malformed/nil payloads → `nil` from the outer `Derive` function (deterministic, explicit fail-on-invalid).

**Tests:** `TestDerive_MalformedPayload` — corrupt JSON → `nil`. `TestDerive_NilPayload` — nil payload → `nil`.

---

## 2. Files Modified

| File | Changes |
|---|---|
| `internal/derive/derive.go` | Added `extractBoolFromPayload`; gated F1/F2 on payload fields; simplified F3; removed dead `fixKeywords` variable |
| `internal/derive/derive_test.go` | Added 6 tests: `NonMaintainerComment`, `UnmergedPR`, `MergedPR`, `MalformedPayload`, `NilPayload`, `ReleaseWithoutKeyword`, `NonMaintainerContradiction`; updated existing tests with `is_maintainer`/`merged` payloads |
| `internal/derive/testdata/maintainer_fixes.json` | Added `is_maintainer: true` |
| `internal/derive/testdata/maintainer_no_regression.json` | Added `is_maintainer: true` |
| `internal/derive/testdata/maintainer_reviewed.json` | Added `is_maintainer: true` |
| `internal/derive/testdata/contradicting_evidence.json` | Added `is_maintainer: true` |
| `internal/derive/testdata/non_maintainer_comment.json` | New — F1 test fixture |
| `internal/derive/testdata/unmerged_pr.json` | New — F2 test fixture |

---

## 3. Tests Added

| Test | Finding | What it proves |
|---|---|---|
| `TestDerive_NonMaintainerComment` | F1 | Non-maintainer comment → `nil` |
| `TestDerive_UnmergedPR` | F2 | Unmerged PR → `nil` |
| `TestDerive_MergedPR` | F2 | Merged PR → `Accommodated` |
| `TestDerive_MalformedPayload` | F4 | Corrupt JSON → `nil` |
| `TestDerive_NilPayload` | F4 | Nil payload → `nil` |
| `TestDerive_ReleaseWithoutKeyword` | F3 | Assertion without "release" → `nil` |
| `TestDerive_NonMaintainerContradiction` | F1 | Non-maintainer contradiction → still emits (bypasses gate) |

---

## 4. Verification Receipts

```
go build ./...                    → exit 0
go vet ./...                      → exit 0
go test ./internal/derive/...     → 16/16 PASS
go test ./internal/normalize/...  → PASS (Wave 1 intact)
go test ./internal/kernel/...     → PASS (kernel intact)
```

---

## 5. Remaining Known Limitations

- `deriveFromGitHubIssue` does not check `is_maintainer` — this is by design per the adversarial review (contradictions bypass the maintainer gate).
- `extractKeywordsFromPayload` still returns `nil` silently on malformed payload — this is acceptable because it's only used for claim formatting, not trust decisions. The trust decisions (`is_maintainer`, `merged`) go through the stricter `extractBoolFromPayload`.
- Rule 4 minor note: the plan's bare "regression" trigger is not implemented as a contradiction keyword — only "reproduces" and "still vulnerable" are matched. This is minor per the review.

---

## 6. What This Does NOT Do

- No changes to Wave 1
- No changes to normalization
- No changes to public API
- No changes to package layout
- No changes to architecture
- No Wave 3 work
- No kernel changes

---

## 7. Next Step

Await Technical Verification after this rework. Do not begin Wave 3.

# Solvent Judge Playground — Implementation Plan (plan4)

**Source:** `plans/demo/plan4_prompt.md`
**Date:** 2026-08-11
**Scope:** Cosmetic cleanup — remove "Transcript written to /dev/null" from judge-facing output. No architecture/kernel/schema changes.

---

## Root cause

`cmd/solvent/main.go:158`:
```go
fmt.Printf("Transcript written to %s\n", *out)
```

This prints to **stdout** when `--out` is specified. Demo scripts use `--out /dev/null` to suppress the full transcript body, but this confirmation leaks to judge-facing output.

## Fix

**Option A (recommended):** Change `fmt.Printf` → `fmt.Fprintf(os.Stderr, ...)` at line 158.

The message moves to stderr. Scripts already suppress stderr via `2>&1 | grep -v '^$'` or similar pipelines. No script changes needed.

**Why this is safe:**
- `cmd/solvent/main.go` is the CLI entry point, not the kernel (`internal/kernel/`)
- Only the diagnostic message moves to stderr; all other output (transcript body, VERDICT) remains on stdout
- Exit codes unchanged; `set -e` behavior unchanged
- The `--out` path still writes the transcript file; only the confirmation message is suppressed

## Files changed

| File | Change |
|---|---|
| `cmd/solvent/main.go` | Line 158: `fmt.Printf` → `fmt.Fprintf(os.Stderr, ...)` |
| `plans/post/playground_implementation_report.md` | Add cleanup note |

## Verification

1. `task demo:track1` → no "Transcript written" in output
2. `task demo:replay` → no "Transcript written" in output
3. `task demo:track2` → no "Transcript written" in output
4. `task inspect` → no "Transcript written" in output
5. All demo results unchanged (3/3/1, audit SAFE, etc.)
6. `task test` — 107 tests pass
7. `grep -Rni "Transcript written to /dev/null" scripts/demo/ README.md` — no matches

## Scope boundaries (unchanged)

- No schema changes
- No kernel changes
- No new features
- No MCP
- No test infrastructure changes
- No Taskfile redesign

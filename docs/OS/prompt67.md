Plan 12 has identified the real root cause correctly: `kernel.TestMain` regenerates a tracked, nondeterministic evidence artifact as a side effect of ordinary `task test`. The generation chain and volatile fields are explicitly traced. 

I would make **three corrections before implementation**.

### 1. The `scripts/m2_accept.sh` claim is internally inconsistent

The plan says:

```text
M2_TRANSCRIPT=1 go test ./kernel/
```

will regenerate the transcript, which is correct. But later it says:

```text
scripts/m2_accept.sh
git status docs/M2_TRANSCRIPT.md
# should show no modification (script manages the file)
```

That is only true if `m2_accept.sh` explicitly restores, normalizes, commits, or otherwise manages the generated artifact. The plan has not yet established that.

The agent should inspect `scripts/m2_accept.sh` fully and determine what happens to the generated transcript after the M2 test completes. Don't assume "script manages the file" merely because it strips volatile sections for comparison.

### 2. Add a deterministic artifact decision

The plan correctly chooses:

> tracked generated evidence, but **not regenerated during ordinary tests**. 

That is a sound model. But the plan should explicitly state:

```text
Ordinary test:
    verify behavior
    MUST NOT mutate tracked evidence

Explicit M2 gate:
    regenerate evidence
    compare/update evidence intentionally
```

This makes the lifecycle contract clear.

Also, `test:m2` should probably invoke the **canonical M2 gate script** if `m2_accept.sh` is already the authoritative workflow, rather than creating a second partially overlapping generation path. Otherwise you risk two subtly different M2 workflows. The existing plan already identifies `m2_accept.sh` as the explicit generation mechanism. 

### 3. Verify the environment-variable semantics

The proposed guard is:

```go
if os.Getenv("M2_TRANSCRIPT") != "" {
```

That means all of these enable generation:

```text
M2_TRANSCRIPT=1
M2_TRANSCRIPT=true
M2_TRANSCRIPT=no
M2_TRANSCRIPT=anything
```

That is probably fine, but the plan should deliberately choose either:

```go
if os.Getenv("M2_TRANSCRIPT") == "1"
```

or explicitly state that non-empty means enabled.

For a test-generation switch, I prefer:

```go
if os.Getenv("M2_TRANSCRIPT") == "1" {
```

It is less surprising and makes the contract precise.

### What I would keep

The core solution is good:

```text id="1rye0z"
task test
    -> does NOT regenerate transcript

explicit M2 gate
    -> DOES regenerate transcript
```

That directly fixes the recurring dirty-tree problem rather than adding `.gitignore` noise or deleting historical evidence. 

The migration and verification steps are also appropriately focused. 

### Final implementation instruction

I would give the coding agent this amendment:

```text
Before implementing Plan 12, resolve these points from the actual repository:

1. Inspect scripts/m2_accept.sh completely.
   Determine exactly what it does with docs/M2_TRANSCRIPT.md after generation.
   Do not assume the script leaves the tree clean.

2. Define the lifecycle contract explicitly:
      task test
          MUST NOT mutate docs/M2_TRANSCRIPT.md

      explicit M2 verification/generation workflow
          MAY regenerate docs/M2_TRANSCRIPT.md intentionally

3. Determine whether scripts/m2_accept.sh should remain the canonical M2
   generation path. Avoid creating a second M2 workflow with subtly different
   behavior. Make test:m2 delegate to the canonical mechanism where practical.

4. Make M2_TRANSCRIPT enablement precise.
   Prefer:
       M2_TRANSCRIPT == "1"
   unless repository conventions establish a different convention.

5. Verify the following sequence after implementation:
      task test
      -> git status remains clean

      explicit M2 generation
      -> transcript intentionally changes or remains byte-identical according
         to the repository's canonical evidence workflow

      scripts/m2_accept.sh
      -> final working-tree behavior is explicitly verified, not assumed

6. Do not change tracking/.gitignore unless repository evidence proves that
   tracked generated evidence is the wrong ownership model.

7. Do not modify docs/M2_TRANSCRIPT.md merely to make tests pass. Preserve its
   current committed evidence unless the explicit M2 workflow intentionally
   produces a new canonical snapshot.
```

**Verdict: APPROVE WITH THESE SMALL PLAN CORRECTIONS.** The root-cause diagnosis is sound, and this is the right fix for the GitHub-publication problem rather than another workaround. 

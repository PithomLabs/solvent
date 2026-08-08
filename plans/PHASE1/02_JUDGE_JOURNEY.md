# 02_JUDGE_JOURNEY.md

**Objective:** Map every path a judge might actually take through the submission,
end to end.

**Audience:** Whoever builds Phase 2/3 against this — it is the contract those
phases implement.

---

## Journey A — Hosted Web Demo (zero clone, one click)

| Step | Time | Action | Failure Mode |
|---|---|---|---|
| 1 | 0:00 | Land on demo URL from submission | Page load fails → show static 1-vs-0 image as fallback |
| 2 | 0:05 | Click "Run the Race" button | Cold start (serverless) → show "warming up" message, ≤15s |
| 3 | 0:20 | Watch identical graphs appear, side by side | Graph fails to render → show pre-recorded animation |
| 4 | 0:40 | Watch falsifying evidence land in both lanes | — |
| 5 | 0:55 | Watch the race fire — both lanes flash | — |
| 6 | 1:05 | Lane A commits (looks fine), Lane B refuses (red error) | — |
| 7 | 1:20 | Narrator explains once | — |
| 8 | 1:40 | AUDIT = 1 vs AUDIT = 0 reveal | — |
| 9 | 1:55 | Zoom out: same rule holds for every node | — |
| 10 | 2:10 | Optional: "See the Repo" link → Journey B entry point | — |
| 11 | 2:15 | Closer + tagline + end card | — |

**Total elapsed:** ≤ 2:30 (leaves room for live Q&A within the 3-minute ceiling)

**Key principle:** The judge never types a command. Everything is one click.

---

## Journey B — Clone and Run (engineer who wants to go deeper)

| Step | Time | Action | Failure Mode |
|---|---|---|---|
| 1 | 0:00 | `git clone` the repo | Slow clone → README has a "just watch" link to Journey A |
| 2 | 0:30 | `task setup` (starts cluster, applies schema) | No Docker → one-line install instructions in README |
| 3 | 1:00 | `task wow` (guided one-shot: setup → demo → race → verify) | Cluster won't start → `task doctor` diagnoses |
| 4 | 1:30 | Watch the race narrated in the terminal | — |
| 5 | 2:00 | See the 1-vs-0 audit reveal in terminal output | — |
| 6 | 2:15 | "I completely understand why this matters" exit state | — |
| 7 | 2:30 | Optional: `task demo` (interactive exploration), read receipts in `docs/` | — |

**Total elapsed:** ≤ 3:00 (clone + setup + race + verification)

**Key principle:** One command does everything. Granular commands exist for
anyone who wants to explore individual stages.

---

## Journey C — Repository Browser (judge who reads code)

| Step | Time | Action | Failure Mode |
|---|---|---|---|
| 1 | 0:00 | Open repo URL from submission | — |
| 2 | 0:10 | Read README hero section | — |
| 3 | 0:25 | Click "receipts" link → see `docs/M0_REPORT.md` | — |
| 4 | 0:45 | Skim M0 transcript: real SQLSTATE errors, real constraint names | — |
| 5 | 1:00 | Read `IMPLEMENTATION_CONTRACT.md` frozen architecture | — |
| 6 | 1:30 | Browse `internal/kernel/` — see 7 functions, zero raw writes | — |
| 7 | 2:00 | Browse `plans/` — see the paper trail from requirements to demo | — |
| 8 | 2:30 | "This team actually built what they claim" exit state | — |

**Total elapsed:** ≤ 3:00

**Key principle:** The repository itself carries credibility. Every claim in the
README traces to a receipt in `docs/`.

---

## Cross-Journey Guarantees

1. **No journey requires typing SQL.** Even Journey B uses `task` commands.
2. **No journey requires domain expertise.** The race is understandable without
   knowing what CVE or KEV means.
3. **Every journey ends at the same reveal:** AUDIT = 1 vs AUDIT = 0.
4. **Failure modes have fallbacks.** Cold start → static image. Clone slow →
   watch link. Cluster won't start → doctor diagnoses.

---

## Acceptance Criteria

- [ ] Every step has an owner journey (A, B, or C), an elapsed-time budget, and
  a named failure mode.
- [ ] Total Journey A time fits inside the 3-minute video ceiling with room for
  live Q&A.
- [ ] No journey requires typing SQL, knowing etcd, or understanding
  serializable isolation.

I read the full research report. **The research is good enough to proceed, but I would NOT hand its implementation specification to the coding agent yet.** There are several important internal inconsistencies that we should resolve first.

The research has found a genuinely strong dataset: GHSA/CVE-2026-33413 + etcd release evidence + the official v3.5 inconsistency postmortem. The external facts also largely check out: CVE-2026-33413 is real and fixed in etcd 3.5.28, and the postmortem documents the v3.5.0 consistency-index corruption and v3.5.3 fix. ([NVD][1])

But there are **four blocking issues**.

## 1. The biggest problem: the falsification scenario isn't actually represented in the fixture

The report says the dataset contains:

> two security/reliability records, three release records, and five normalized evidence items. 

But the actual proposed JSON fixture contains only:

* GHSA
* v3.5.27
* v3.5.28
* postmortem
* synthetic contradiction

That's **five records**, but only **two release records**: 3.5.27 and 3.5.28. The supposed v3.5.0 release record is missing. 

Yet Scenario D explicitly requires:

> `ev_rel_v350`

and a belief:

> `bel_v350_stable`

followed by:

> `act_deploy_v350`

before the postmortem retracts it. 

So the report currently describes **two different datasets**.

### Fix

Add the actual v3.5.0 release record **if we really need Scenario D**, or redesign Scenario D around the records actually present.

I lean toward **adding v3.5.0**. It's tiny, authoritative, and makes the falsification story much more natural.

---

## 2. The postmortem does not literally say "v3.5.0 was unsuitable for production"

This is subtler and more important conceptually.

The report creates:

> `bel_v350_stable`: "etcd v3.5.0 is General Availability and suitable for production deployments"

and then treats the postmortem as invalidating that belief. 

But the actual postmortem says something more nuanced: v3.5.0 had a critical data inconsistency bug, but the authors state they were **not aware of users reporting production data corruption**; it nevertheless motivated a public statement and affected trust in reliability. ([Fossies][2])

So we shouldn't have Solvent claim:

> "The evidence proves v3.5.0 was not production-ready."

That's stronger than the source.

The better belief is something like:

> **"etcd v3.5.0 has a documented data-consistency defect under specified crash conditions."**

Then the postmortem is **direct evidence for the belief**, rather than retroactively falsifying some exaggerated "safe for production" claim.

But that changes the demo semantics.

We need to decide whether Scenario D is demonstrating:

**A. true factual falsification**

```text
belief: v3.5.0 is safe
        ↓
new evidence says unsafe
        ↓
RETRACT
```

or the more defensible:

**B. decision invalidation**

```text
belief: v3.5.0 is suitable for deployment
        ↓
new evidence reveals material reliability defect
        ↓
deployment decision becomes invalid
        ↓
RETRACT
```

**B is much better.**

It demonstrates exactly what Solvent is supposed to do: **invalidate an action decision when the evidence supporting that decision changes**, rather than pretending the underlying historical fact itself was false.

---

## 3. The `belief_edge` mapping appears wrong

This line in the implementation specification worries me:

> `parent_belief_id = 'ev_postmortem_v35'` 

`ev_postmortem_v35` is an **evidence ID**, not a belief ID.

If `belief_edge` really represents relationships between beliefs—as the report repeatedly describes—then this is conceptually wrong.

The report earlier says:

> evidence supports derived belief via a SUPPORTS edge in `belief_edge` 

So we need to establish the actual existing Solvent edge semantics before writing the fixture implementation specification.

**Do not let the coding agent invent a mapping here.**

This is exactly the sort of thing we wanted the research phase to catch.

---

## 4. The research report is slightly overconfident about what the existing code already does

For example:

> "The presence of an active, unresolved CONTRADICTS edge automatically freezes belief promotion rules." 

And:

> "ingested release metadata ... satisfies the Fix Availability Debt" 

Those may indeed be capabilities in Solvent, but the research agent sometimes writes them as though they are **requirements guaranteed by the existing architecture**, rather than carefully distinguishing:

```text
existing Solvent behavior
vs
proposed fixture behavior
vs
desired demo behavior
```

That's dangerous because we're now at the implementation stage.

The report needs one final reconciliation against the **actual current repository**.

---

# What I think of the research overall

### Research quality: **8/10**

The **data selection is excellent**.

The GHSA is particularly good because it gives us a very clean:

```text
v3.5.27
   ↓
affected by CVE-2026-33413
   ↓
v3.5.28
   ↓
fixed
```

The official changelog independently confirms that v3.5.28 fixed the authorization bypass. ([GitHub][3])

The postmortem is also an unusually good choice for the second story because it is **maintainer-authored, historical, concrete, and directly about distributed-state correctness**, rather than being another generic CVE. ([Fossies][2])

And I like the deliberate rejection of NVD/OSV as runtime inputs. The report correctly converges on:

```text
authoritative source
       ↓
pinned snapshot
       ↓
offline deterministic Solvent
```

rather than turning the demo into an API integration project. 

### But implementation readiness: **6/10**

It isn't quite implementation-ready because of the contradictions above.

---

# What I want to do next

**Do NOT send the whole implementation specification to the coding agent yet.**

Instead, give **Fable/Claude one narrow adversarial review** of this research report.

Not a code review.

Ask:

> **"Does this proposed real-world dataset actually correspond to the semantics and capabilities of the frozen Solvent repository?"**

Specifically have it check:

1. Does the proposed five-record fixture actually support all claimed scenarios?
2. Should v3.5.0 be added?
3. Is `bel_v350_stable` a defensible belief from the source?
4. Is the postmortem legitimately an `INVALIDATES` event or merely supporting evidence for a new belief?
5. Are the proposed `belief_edge` directions/IDs compatible with the existing schema?
6. Does the GHSA → v3.5.27 → v3.5.28 debt/promotion story match actual Solvent semantics?
7. Which parts of the research report are **facts about existing Solvent**, versus proposed implementation?
8. What is the **minimum correction** needed to make the dataset implementation-ready?

Then **we make one final data-spec decision and hand it to the coding agent.**

This is worth one adversarial pass because we're at the exact boundary between **research and implementation**. The report found the right raw material; we just need to make sure we aren't quietly asking the coding agent to implement semantics that the research report invented.

And importantly, **I would not broaden the research any further.** The dataset choice itself is strong. We need reconciliation, not more research.

[1]: https://nvd.nist.gov/vuln/detail/CVE-2026-33413?utm_source=chatgpt.com "NVD - CVE-2026-33413"
[2]: https://fossies.org/linux/etcd/Documentation/postmortems/v3.5-data-inconsistency.md?utm_source=chatgpt.com "etcd: .../v3.5-data-inconsistency.md | Fossies"
[3]: https://github.com/etcd-io/etcd/blob/main/CHANGELOG/CHANGELOG-3.5.md?utm_source=chatgpt.com "etcd/CHANGELOG/CHANGELOG-3.5.md at main · etcd-io/etcd · GitHub"


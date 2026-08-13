# Attribution

## Track 1 — CVE Lifecycle

### GHSA-q8m4-xhhv-38mg (github_advisory)
- **Source:** GitHub Security Advisories
- **URL:** https://github.com/advisories/GHSA-q8m4-xhhv-38mg
- **License:** Data is provided under the [GitHub Advisory Database terms](https://github.com/github/advisory-database/blob/main/LICENSE.md) (CC-BY-4.0)

### etcd v3.5.27 release (release)
- **Source:** etcd-io/etcd GitHub Releases
- **URL:** https://github.com/etcd-io/etcd/releases/tag/v3.5.27
- **License:** Apache License 2.0

### etcd v3.5.28 release (release)
- **Source:** etcd-io/etcd GitHub Releases
- **URL:** https://github.com/etcd-io/etcd/releases/tag/v3.5.28
- **License:** Apache License 2.0

## Track 2 — Historical Retraction

### etcd v3.5.0 release (release)
- **Source:** etcd-io/etcd GitHub Releases
- **URL:** https://github.com/etcd-io/etcd/releases/tag/v3.5.0
- **License:** Apache License 2.0

### etcd v3.5 Data Inconsistency Postmortem (postmortem)
- **Source:** etcd community documentation
- **URL:** https://github.com/etcd-io/etcd/blob/main/Documentation/postmortems/v3.5-data-inconsistency.md
- **License:** Apache License 2.0

## Corpus — Institutional Memory (Phase 3)

### etcd issue corpus (github_issue)
- **Source:** etcd-io/etcd GitHub Issues, via the REST endpoint `/repos/etcd-io/etcd/issues`
- **URL:** https://github.com/etcd-io/etcd/issues
- **License:** the etcd project is licensed under the Apache License 2.0.

**A distinction the entries above do not have to make.** Those are project artifacts —
releases, advisories, maintainer documentation — squarely covered by the project's own
licence. An issue corpus is different: the *titles and bodies are user-contributed
content*, submitted by thousands of individual reporters under
[GitHub's Terms of Service](https://docs.github.com/site-policy/github-terms/github-terms-of-service),
not assigned to the etcd project under Apache-2.0. The repository's licence covers the
code, not every comment written in its issue tracker.

That is why the corpus is used the way it is: as retrieval material that is read,
cited by URL, and attributed back to its source issue — never republished as though it
were the project's own text, and never presented as anyone's authored work but the
reporter's.

**Not committed.** The snapshot is fetched, not vendored. `corpus-data/` is gitignored
and reproducible with `task corpus:fetch`; `corpus-data/etcd-issues.ndjson.meta.json`
records the capture timestamp, the exact API parameters, the counts, and a SHA-256 of
the NDJSON so a given corpus state can be cited precisely.

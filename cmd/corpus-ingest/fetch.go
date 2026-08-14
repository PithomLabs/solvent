package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	githubAPI = "https://api.github.com"
	// Ascending by creation time. GitHub's default ordering is by recency, which
	// moves under a long walk; creation time never changes. The cursor pagination
	// below is what actually guarantees page boundaries hold, but a stable sort key
	// costs nothing and keeps the walk deterministic across re-captures.
	listParams = "state=all&per_page=100&sort=created&direction=asc"
)

// ghIssue is the subset of GitHub's issue payload this command reads.
//
// PullRequest is the discriminator: GitHub's issues endpoint returns pull
// requests interleaved with issues, and the ONLY reliable way to tell them apart
// is the presence of this key. etcd has ~14,378 PRs against ~7,239 issues, so an
// unfiltered ingest would be roughly three quarters wrong.
type ghIssue struct {
	Number    int     `json:"number"`
	Title     string  `json:"title"`
	Body      *string `json:"body"`
	State     string  `json:"state"`
	HTMLURL   string  `json:"html_url"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
	ClosedAt  *string `json:"closed_at"`
	Labels    []struct {
		Name string `json:"name"`
	} `json:"labels"`
	PullRequest *json.RawMessage `json:"pull_request"`
}

// githubToken resolves the credential without ever touching a file in the repo
// or the working tree.
//
// Order: an already-exported GITHUB_TOKEN, else the authenticated gh session.
// There is deliberately no --token flag — flags land in shell history and in the
// output of ps — and deliberately no .env read, so nothing here depends on a
// secret living beside the source. A third party with their own `gh` login can
// reproduce the capture with no repository-resident credential.
//
// The token is held in memory for the process lifetime and is never printed,
// logged, or written anywhere.
func githubToken() (string, error) {
	if t := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); t != "" {
		return t, nil
	}
	out, err := exec.Command("gh", "auth", "token").Output()
	if err != nil {
		return "", fmt.Errorf("no GITHUB_TOKEN in the environment and `gh auth token` failed: %w\n"+
			"  fix: gh auth login   (or) export GITHUB_TOKEN=<token>", err)
	}
	t := strings.TrimSpace(string(out))
	if t == "" {
		return "", fmt.Errorf("`gh auth token` returned nothing; run `gh auth login`")
	}
	return t, nil
}

func runFetch(ctx context.Context, repo, outPath string) error {
	token, err := githubToken()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)

	fetchedAt := time.Now().UTC().Format(time.RFC3339)
	client := &http.Client{Timeout: 60 * time.Second}

	var pages, itemsSeen, prs, kept, open, closed int
	seen := map[int]bool{}

	fmt.Printf("repository: %s\n", repo)
	fmt.Printf("endpoint:   /repos/%s/issues\n", repo)
	fmt.Printf("params:     %s\n", listParams)
	fmt.Printf("fetched_at: %s\n", fetchedAt)

	// Cursor pagination, driven by the Link: rel="next" header.
	//
	// NOT page=N. GitHub refuses that beyond ~10,000 items on this endpoint with
	// HTTP 422 ("Pagination with the page parameter is not supported for large
	// datasets, please use cursor based pagination"), and etcd's ~21,600 issues +
	// PRs is well past that line. Following the server's own cursor also removes
	// the offset-drift hazard entirely: each page names its successor, so a row
	// inserted mid-walk cannot shift a boundary underneath us.
	next := fmt.Sprintf("%s/repos/%s/issues?%s", githubAPI, repo, listParams)
	for next != "" {
		batch, link, err := getPage(ctx, client, token, next)
		if err != nil {
			_ = w.Flush()
			_ = f.Close()
			return err
		}
		next = nextLink(link)
		if len(batch) == 0 {
			break
		}
		pages++
		itemsSeen += len(batch)

		for _, g := range batch {
			if g.PullRequest != nil {
				prs++
				continue
			}
			// A duplicate here means pagination drifted despite the stable sort.
			// Surfacing it is the point: silently deduping would hide the defect.
			if seen[g.Number] {
				continue
			}
			seen[g.Number] = true

			body := ""
			if g.Body != nil {
				body = *g.Body
			}
			labels := make([]string, 0, len(g.Labels))
			for _, l := range g.Labels {
				labels = append(labels, l.Name)
			}

			iss := SnapshotIssue{
				Number:    g.Number,
				Title:     g.Title,
				Body:      body,
				State:     g.State,
				URL:       g.HTMLURL,
				CreatedAt: g.CreatedAt,
				UpdatedAt: g.UpdatedAt,
				ClosedAt:  g.ClosedAt,
				Labels:    labels,
			}
			line, err := json.Marshal(iss)
			if err != nil {
				_ = w.Flush()
				_ = f.Close()
				return err
			}
			if _, err := w.Write(append(line, '\n')); err != nil {
				_ = w.Flush()
				_ = f.Close()
				return err
			}
			kept++
			switch g.State {
			case "open":
				open++
			case "closed":
				closed++
			}
		}

		if pages%25 == 0 {
			fmt.Printf("  … page %d  items=%d  issues=%d  prs=%d\n", pages, itemsSeen, kept, prs)
		}
	}

	if err := w.Flush(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	digest, err := fileSHA256(outPath)
	if err != nil {
		return err
	}

	meta := SnapshotMeta{
		FetchedAt:            fetchedAt,
		Repository:           repo,
		Endpoint:             fmt.Sprintf("/repos/%s/issues", repo),
		Params:               listParams,
		PagesFetched:         pages,
		ItemsSeen:            itemsSeen,
		PullRequestsExcluded: prs,
		IssuesKept:           kept,
		OpenCount:            open,
		ClosedCount:          closed,
		NDJSONSHA256:         digest,
		Note: "Captured API snapshot, not an atomic one: GitHub may change while pagination runs. " +
			"Ordering is by ascending creation time, which never changes, so page boundaries stay stable. " +
			"This NDJSON and this digest are the canonical Phase 3 corpus; both local and cloud ingestion consume it.",
	}
	if err := writeMeta(outPath, meta); err != nil {
		return err
	}

	fmt.Printf("PAGES_FETCHED=%d\n", pages)
	fmt.Printf("ITEMS_SEEN=%d\n", itemsSeen)
	fmt.Printf("PULL_REQUESTS_EXCLUDED=%d\n", prs)
	fmt.Printf("ISSUES_KEPT=%d\n", kept)
	fmt.Printf("OPEN=%d\n", open)
	fmt.Printf("CLOSED=%d\n", closed)
	fmt.Printf("NDJSON_SHA256=%s\n", digest)
	fmt.Printf("SNAPSHOT=%s\n", outPath)
	fmt.Printf("META=%s\n", metaPathFor(outPath))

	if open+closed != kept {
		fmt.Printf("VERDICT: FAIL (open+closed=%d != issues_kept=%d)\n", open+closed, kept)
		return fmt.Errorf("snapshot is internally inconsistent")
	}
	fmt.Println("VERDICT: PASS")
	return nil
}

// getPage performs one request, honouring GitHub's rate limits rather than
// hammering through them.
func getPage(ctx context.Context, client *http.Client, token, url string) ([]ghIssue, string, error) {
	const maxAttempts = 5
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, "", err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
		req.Header.Set("User-Agent", "solvent-corpus-ingest")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(backoff(attempt))
			continue
		}

		switch {
		case resp.StatusCode == http.StatusOK:
			var out []ghIssue
			dec := json.NewDecoder(resp.Body)
			err := dec.Decode(&out)
			link := resp.Header.Get("Link")
			_ = resp.Body.Close()
			if err != nil {
				return nil, "", fmt.Errorf("decode page: %w", err)
			}
			// Primary rate limit: pause before the window is exhausted rather
			// than after being refused.
			if rem := resp.Header.Get("X-RateLimit-Remaining"); rem != "" {
				if n, e := strconv.Atoi(rem); e == nil && n <= 2 {
					waitForReset(resp.Header.Get("X-RateLimit-Reset"))
				}
			}
			return out, link, nil

		case resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests:
			// Secondary rate limit or abuse detection.
			retryAfter := resp.Header.Get("Retry-After")
			reset := resp.Header.Get("X-RateLimit-Reset")
			_ = resp.Body.Close()
			if retryAfter != "" {
				if n, e := strconv.Atoi(retryAfter); e == nil {
					fmt.Printf("  rate limited; sleeping %ds (Retry-After)\n", n)
					time.Sleep(time.Duration(n) * time.Second)
					continue
				}
			}
			if reset != "" {
				waitForReset(reset)
				continue
			}
			lastErr = fmt.Errorf("HTTP %d with no retry hint", resp.StatusCode)
			time.Sleep(backoff(attempt))

		default:
			b := make([]byte, 512)
			n, _ := resp.Body.Read(b)
			_ = resp.Body.Close()
			// The URL carries no credential; the token travels in a header.
			return nil, "", fmt.Errorf("HTTP %d from GitHub: %s", resp.StatusCode, strings.TrimSpace(string(b[:n])))
		}
	}
	return nil, "", fmt.Errorf("giving up after %d attempts: %w", maxAttempts, lastErr)
}

// nextLink extracts the rel="next" URL from a GitHub Link header, or "" when the
// walk is finished. GitHub emits the cursor form here, e.g.
//
//	<https://api.github.com/repositories/11225014/issues?...&after=Y3Vyc29yOnYyOpHOAA>; rel="next"
//
// so following it is what keeps this fetch off the page= path GitHub rejects at
// depth.
func nextLink(header string) string {
	for _, part := range strings.Split(header, ",") {
		seg := strings.Split(strings.TrimSpace(part), ";")
		if len(seg) < 2 {
			continue
		}
		isNext := false
		for _, attr := range seg[1:] {
			if strings.TrimSpace(attr) == `rel="next"` {
				isNext = true
				break
			}
		}
		if !isNext {
			continue
		}
		u := strings.TrimSpace(seg[0])
		u = strings.TrimPrefix(u, "<")
		u = strings.TrimSuffix(u, ">")
		if u != "" {
			return u
		}
	}
	return ""
}

func backoff(attempt int) time.Duration {
	return time.Duration(1<<attempt) * time.Second
}

func waitForReset(reset string) {
	n, err := strconv.ParseInt(reset, 10, 64)
	if err != nil {
		time.Sleep(60 * time.Second)
		return
	}
	d := time.Until(time.Unix(n, 0)) + 2*time.Second
	if d <= 0 {
		return
	}
	fmt.Printf("  rate limit window exhausted; sleeping %s\n", d.Round(time.Second))
	time.Sleep(d)
}

package wizard

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"html/template"
	"log"
	"net/http"
	"regexp"
	"time"

	"github.com/PithomLabs/solvent/proof"
)

// Mount point.
//
// The settled contract put the wizard at `/` with the API at `/api/*`. It is mounted
// here instead, and `/` keeps serving the existing landing page, because the recorded
// live URL is the one a judge will open and it must not become a wizard whose search
// cannot work until an App Runner instance role exists. The API moves with the wizard
// so the whole feature is one self-contained mount; every fetch below uses a relative
// path, so promoting it to `/` later is a change to this constant and nothing else.
const (
	Prefix     = "/demo"
	cookieName = "solvent_wizard"

	// ProofPath sits at the root, not under Prefix. The contract links it from screen
	// 3's footer as /proof and treats it as evidence rather than as a wizard screen: it
	// reads no scenario, sets no cookie, and touches no database.
	ProofPath = "/proof"
)

//go:embed templates/*.html
var templateFS embed.FS

// Embedded, not glob-loaded from disk.
//
// demo/cloud/web parses its templates through a working-directory-relative glob, which
// works in the container and breaks in a test. Embedding removes the working directory
// from the problem entirely and means the Dockerfile needs no new COPY.
var shell = template.Must(template.ParseFS(templateFS, "templates/*.html"))

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// Routes registers the wizard on mux. Nothing outside Prefix is touched.
func (s *Server) Routes(mux *http.ServeMux) {
	mux.HandleFunc(Prefix, s.handleShell)
	mux.HandleFunc(Prefix+"/", s.handleShell)
	mux.HandleFunc(Prefix+"/api/state", s.handleState)
	mux.HandleFunc(Prefix+"/api/search", s.handleSearch)
	mux.HandleFunc(Prefix+"/api/select", s.handleSelect)
	mux.HandleFunc(Prefix+"/api/promote", s.handlePromote)
	mux.HandleFunc(Prefix+"/api/authorize", s.handleAuthorize)
	mux.HandleFunc(Prefix+"/api/discharge", s.handleDischarge)
	mux.HandleFunc(Prefix+"/api/introduce", s.handleIntroduce)
	mux.HandleFunc(Prefix+"/api/retract", s.handleRetract)
	mux.HandleFunc(Prefix+"/api/reset", s.handleReset)
	mux.HandleFunc(ProofPath, s.handleProof)
}

// handleProof renders the isolation evidence. Static, stateless, no database.
func (s *Server) handleProof(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := shell.ExecuteTemplate(w, "proof.html", map[string]string{
		"Isolation": proof.Isolation,
		"Act6Tier":  proof.Act6Tier,
	}); err != nil {
		log.Printf("wizard: render proof: %v", err)
	}
}

// shellData is what the HTML shell needs before any fetch happens.
type shellData struct {
	Prefix          string
	DefaultQuestion string
	Steps           []string
}

func (s *Server) handleShell(w http.ResponseWriter, r *http.Request) {
	// The scenario is established here so the first /api/state call has one to read.
	if _, err := s.scenario(r.Context(), w, r); err != nil {
		log.Printf("wizard: shell: %v", err)
		http.Error(w, "the wizard could not prepare a scenario", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := shell.ExecuteTemplate(w, "shell.html", shellData{
		Prefix:          Prefix,
		DefaultQuestion: DefaultQuestion,
		Steps:           []string{ScreenAsk, ScreenDischarge, ScreenFalsify},
	}); err != nil {
		log.Printf("wizard: render shell: %v", err)
	}
}

// scenario resolves the judge's scenario, seeding lazily on first contact.
//
// Lazy and per-visitor on purpose. solvent-init seeds Track 2 at deploy time; a second
// seeder running there would race it and collide. This one runs on first request, mints
// its own scenario id, and never truncates anything — in particular it never calls
// resetAndSeed, whose TRUNCATE is not scenario-scoped and would take every other
// judge's scenario with it.
func (s *Server) scenario(ctx context.Context, w http.ResponseWriter, r *http.Request) (string, error) {
	if c, err := r.Cookie(cookieName); err == nil && uuidRe.MatchString(c.Value) {
		var n int
		if err := s.db.QueryRowContext(ctx,
			`SELECT count(*) FROM belief WHERE scenario_id = $1::UUID`, c.Value).Scan(&n); err == nil && n > 0 {
			return c.Value, nil
		}
	}
	sid, err := s.NewScenario(ctx)
	if err != nil {
		return "", err
	}
	if err := s.Seed(ctx, sid); err != nil {
		return "", err
	}
	s.setCookie(w, sid)
	return sid, nil
}

func (s *Server) setCookie(w http.ResponseWriter, sid string) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    sid,
		Path:     Prefix,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(7 * 24 * time.Hour),
	})
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	sid, err := s.scenario(r.Context(), w, r)
	if err != nil {
		writeErr(w, http.StatusServiceUnavailable, err)
		return
	}
	st, err := s.State(r.Context(), sid)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

type searchReq struct {
	Query string `json:"query"`
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, errors.New("POST only"))
		return
	}
	sid, bid, ok := s.resolve(w, r)
	if !ok {
		return
	}
	var req searchReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	_ = sid

	// Bounded independently of the server's write timeout: corpus.Embed backs off for
	// up to ~62s on throttling, which is correct for a batch job and far too long for a
	// judge waiting on a click. Embed honours ctx during its backoff.
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	res, err := s.Search(ctx, bid, req.Query)
	if err != nil {
		// A missing Bedrock permission is a deployment fact, not a server fault, and the
		// message says which permission. It is never dressed up as an empty result set.
		status := http.StatusBadGateway
		if errors.Is(err, ErrEmbedderUnavailable) {
			status = http.StatusServiceUnavailable
		}
		writeErr(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

type selectReq struct {
	CorpusID string  `json:"corpus_id"`
	Query    string  `json:"query"`
	Distance float64 `json:"distance"`
	On       bool    `json:"on"`
}

func (s *Server) handleSelect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, errors.New("POST only"))
		return
	}
	_, bid, ok := s.resolve(w, r)
	if !ok {
		return
	}
	var req selectReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if !uuidRe.MatchString(req.CorpusID) {
		writeErr(w, http.StatusBadRequest, errors.New("corpus_id must be a UUID"))
		return
	}
	if err := s.Select(r.Context(), bid, req.CorpusID, req.Query, req.Distance, req.On); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handlePromote(w http.ResponseWriter, r *http.Request) {
	s.verdictHandler(w, r, func(ctx context.Context, sid, bid string) Verdict {
		return s.Promote(ctx, sid, bid)
	})
}

func (s *Server) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	s.verdictHandler(w, r, func(ctx context.Context, sid, bid string) Verdict {
		return s.Authorize(ctx, sid, bid)
	})
}

type dischargeReq struct {
	Check    string `json:"check"`
	Artifact string `json:"artifact"`
}

func (s *Server) handleDischarge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, errors.New("POST only"))
		return
	}
	sid, bid, ok := s.resolve(w, r)
	if !ok {
		return
	}
	var req dischargeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	// A refusal is a 200 carrying a verdict, not an HTTP error. The database refusing is
	// a normal, expected, load-bearing outcome of this demo; making it a 4xx would
	// invite a client to treat it as a bug and retry or hide it.
	writeJSON(w, http.StatusOK, s.Discharge(r.Context(), sid, bid, req.Check, req.Artifact))
}

type introduceReq struct {
	IssueURL string `json:"issue_url"`
}

func (s *Server) handleIntroduce(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, errors.New("POST only"))
		return
	}
	sid, bid, ok := s.resolve(w, r)
	if !ok {
		return
	}
	var req introduceReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	// Bounded like search: introducing measures a distance, which means one Bedrock call.
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	intro, err := s.Introduce(ctx, sid, bid, req.IssueURL)
	if err != nil {
		// A rejected URL is the judge's typo, not a server fault.
		status := http.StatusBadRequest
		if errors.Is(err, ErrEmbedderUnavailable) {
			status = http.StatusServiceUnavailable
		}
		writeErr(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, intro)
}

type retractReq struct {
	Mode string `json:"mode"`
}

func (s *Server) handleRetract(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, errors.New("POST only"))
		return
	}
	sid, bid, ok := s.resolve(w, r)
	if !ok {
		return
	}
	var req retractReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	// A refusal is a 200 carrying a verdict. On this screen the refusal IS the result.
	writeJSON(w, http.StatusOK, s.Retract(r.Context(), sid, bid, req.Mode))
}

func (s *Server) handleReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, errors.New("POST only"))
		return
	}
	// A new scenario and a fresh insert. The previous one is abandoned, not deleted:
	// TRUNCATE here would be unscoped and would destroy every other judge's run.
	sid, err := s.NewScenario(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if err := s.Seed(r.Context(), sid); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	s.setCookie(w, sid)
	writeJSON(w, http.StatusOK, map[string]string{"scenario_id": sid})
}

// verdictHandler is the shared shape for the two statements that must stay identical
// across screens. One body, one call site each, no screen parameter anywhere.
func (s *Server) verdictHandler(w http.ResponseWriter, r *http.Request, fn func(context.Context, string, string) Verdict) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, errors.New("POST only"))
		return
	}
	sid, bid, ok := s.resolve(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, fn(r.Context(), sid, bid))
}

// resolve produces the scenario and the belief the judge is driving.
func (s *Server) resolve(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	sid, err := s.scenario(r.Context(), w, r)
	if err != nil {
		writeErr(w, http.StatusServiceUnavailable, err)
		return "", "", false
	}
	st, err := s.State(r.Context(), sid)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return "", "", false
	}
	return sid, st.BeliefID, true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("wizard: encode response: %v", err)
	}
}

func writeErr(w http.ResponseWriter, status int, err error) {
	log.Printf("wizard: %d: %v", status, err)
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

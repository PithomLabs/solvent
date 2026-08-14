package corpus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/smithy-go"
)

// Amazon Titan Text Embeddings V2, verified available ON_DEMAND in us-west-2.
//
// These three values together are the embedding specification. Change any one of
// them and every stored vector becomes stale in a way nothing in the database can
// detect, because a 1024-float column accepts a vector from any model. They are
// therefore recorded into the run's checkpoint metadata, not just used.
const (
	DefaultEmbedModel  = "amazon.titan-embed-text-v2:0"
	DefaultEmbedRegion = "us-west-2"

	// EmbedNormalize=true makes Titan return unit vectors, which is what the corpus
	// index's vector_cosine_ops opclass expects. Verified against the live model:
	// the returned vector has L2 norm 1.0.
	//
	// Exported because it is part of the embedding specification the run records in
	// its checkpoint metadata. A sidecar that restates this as a literal can drift
	// from what was actually sent; one that reads it here cannot.
	EmbedNormalize = true

	// MaxEmbedChars bounds the text sent per issue.
	//
	// Titan v2 accepts roughly 8k tokens. etcd issue bodies routinely carry stack
	// traces, full logs and pasted YAML — the ingest scanner is sized to 16 MB for
	// exactly that reason — so an unbounded body would be refused by the model on a
	// noticeable fraction of the corpus and would spend tokens on log noise that
	// carries little retrieval signal. Truncating at a fixed, documented budget
	// makes every issue's cost predictable and every vector reproducible.
	MaxEmbedChars = 6000
)

// BuildEmbedText composes the exact text embedded for one corpus row.
//
// This function IS the embedding input specification. A change here silently
// invalidates every stored vector, so TestGoldenEmbedText pins its output the same
// way TestGoldenContentHash pins the corpus hash.
//
// Title and body are joined because they carry different signal: the title is the
// human summary, the body is where the symptom actually appears. Retrieval for a
// question like "is etcd v3.5.x safe to deploy?" depends on the body — etcd titles
// are frequently terse enough to be useless on their own.
//
// Truncation cuts on a rune boundary. Slicing a UTF-8 string by byte count can
// split a multi-byte character and produce invalid UTF-8, which the model would
// either reject or silently mangle.
func BuildEmbedText(title, body string) string {
	text := strings.TrimSpace(title)
	if b := strings.TrimSpace(body); b != "" {
		text = text + "\n\n" + b
	}
	if len(text) <= MaxEmbedChars {
		return text
	}
	// Walk back to the last valid rune boundary at or before the budget.
	cut := MaxEmbedChars
	for cut > 0 && !utf8.RuneStart(text[cut]) {
		cut--
	}
	return text[:cut]
}

// Embedder produces vectors from text using Amazon Bedrock.
//
// Credentials come from the ambient AWS credential chain — environment, shared
// config, instance role — and never from a flag or a repo-resident file, matching
// the credential discipline the GitHub fetcher already follows.
type Embedder struct {
	client    *bedrockruntime.Client
	modelID   string
	region    string
	dimension int
}

// NewEmbedder resolves AWS configuration and returns a client.
//
// It does not verify model access; the first Embed call does that, and its error
// is the honest place for an access failure to surface.
func NewEmbedder(ctx context.Context, region, modelID string) (*Embedder, error) {
	if region == "" {
		region = DefaultEmbedRegion
	}
	if modelID == "" {
		modelID = DefaultEmbedModel
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}
	return &Embedder{
		client:    bedrockruntime.NewFromConfig(cfg),
		modelID:   modelID,
		region:    region,
		dimension: Dim,
	}, nil
}

func (e *Embedder) ModelID() string { return e.modelID }
func (e *Embedder) Region() string  { return e.region }

type titanRequest struct {
	InputText  string `json:"inputText"`
	Dimensions int    `json:"dimensions"`
	Normalize  bool   `json:"normalize"`
}

type titanResponse struct {
	Embedding           []float32 `json:"embedding"`
	InputTextTokenCount int       `json:"inputTextTokenCount"`
}

// Embed returns the vector for text, plus the model's input token count.
//
// Retries on throttling with exponential backoff, following the same shape as the
// GitHub fetcher: bounded attempts, and a refusal that is reported rather than
// papered over. A width other than Dim is a hard error — never padded, never
// truncated — because a silently reshaped vector would corrupt the corpus in a way
// no downstream check could detect.
func (e *Embedder) Embed(ctx context.Context, text string) ([]float32, int, error) {
	body, err := json.Marshal(titanRequest{
		InputText:  text,
		Dimensions: e.dimension,
		Normalize:  EmbedNormalize,
	})
	if err != nil {
		return nil, 0, err
	}

	const maxAttempts = 6
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		out, err := e.client.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
			ModelId:     aws.String(e.modelID),
			ContentType: aws.String("application/json"),
			Accept:      aws.String("application/json"),
			Body:        body,
		})
		if err == nil {
			var resp titanResponse
			if err := json.Unmarshal(out.Body, &resp); err != nil {
				return nil, 0, fmt.Errorf("decode Titan response: %w", err)
			}
			if len(resp.Embedding) != e.dimension {
				return nil, 0, fmt.Errorf(
					"Titan returned %d dimensions, want %d: the model or its parameters changed",
					len(resp.Embedding), e.dimension)
			}
			return resp.Embedding, resp.InputTextTokenCount, nil
		}

		lastErr = err
		// An expired or revoked session is not a transient condition. Retrying it
		// 7,000 times produces a long silence that looks exactly like throttling,
		// which is what it looked like the first time this happened. Fail fast and
		// say what to do about it.
		if isAuthFailure(err) {
			return nil, 0, fmt.Errorf(
				"AWS credentials are expired or invalid: %w\n"+
					"  fix: aws login   (the session is short-lived; the embedding checkpoint\n"+
					"       preserves work already done, so a re-run resumes without re-billing)", err)
		}
		// A rejected input is also permanent, but it is not a credential problem and
		// must not be reported as one: telling an operator to run `aws login` when the
		// real fault is an over-long or malformed body sends them to the wrong place.
		if isInvalidInput(err) {
			return nil, 0, fmt.Errorf(
				"Bedrock rejected the request as invalid (not a credential problem): %w\n"+
					"  check: the model ID, the request parameters, and whether the text exceeds\n"+
					"         the model's input limit (see corpus.MaxEmbedChars)", err)
		}
		if !isThrottling(err) || attempt == maxAttempts {
			return nil, 0, fmt.Errorf("bedrock invoke (%s): %w", e.modelID, err)
		}
		select {
		case <-ctx.Done():
			return nil, 0, ctx.Err()
		case <-time.After(time.Duration(1<<attempt) * time.Second):
		}
	}
	return nil, 0, fmt.Errorf("bedrock invoke gave up after %d attempts: %w", maxAttempts, lastErr)
}

// isAuthFailure reports whether err means the AWS session is unusable, as opposed
// to the service being busy. These are opposite situations: one is worth waiting
// out, the other never resolves on its own.
func isAuthFailure(err error) bool {
	var ae smithy.APIError
	if errors.As(err, &ae) {
		switch ae.ErrorCode() {
		case "ExpiredToken", "ExpiredTokenException", "InvalidSignatureException",
			"UnrecognizedClientException", "AccessDeniedException":
			return true
		}
	}
	// Credential resolution fails before any API call and carries no SMITHY code.
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "expired") ||
		strings.Contains(msg, "failed to refresh cached credentials") ||
		strings.Contains(msg, "authorization grant is invalid")
}

// isInvalidInput reports whether Bedrock refused the request itself — a bad model
// ID, a malformed body, or text past the model's input limit.
//
// This is deliberately separate from isAuthFailure. Both are permanent, so both
// skip the retry loop, but they send the operator to opposite places: one to the
// credential chain, the other to the request. ValidationException used to be
// classified as an auth failure, which meant a body too long for Titan surfaced as
// "run aws login" — advice that could never fix it.
func isInvalidInput(err error) bool {
	var ae smithy.APIError
	if errors.As(err, &ae) {
		switch ae.ErrorCode() {
		case "ValidationException", "SerializationException",
			"ResourceNotFoundException", "ModelNotReadyException":
			return true
		}
	}
	return false
}

// isThrottling reports whether err is Bedrock backpressure rather than a real
// failure. Typed through smithy rather than matched on message text.
func isThrottling(err error) bool {
	var ae smithy.APIError
	if errors.As(err, &ae) {
		switch ae.ErrorCode() {
		case "ThrottlingException", "TooManyRequestsException", "ServiceUnavailableException":
			return true
		}
	}
	return false
}

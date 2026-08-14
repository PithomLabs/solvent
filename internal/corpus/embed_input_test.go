package corpus

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// TestGoldenEmbedText pins the embedding input specification.
//
// BuildEmbedText's output IS what gets sent to Titan, so this string determines
// every stored vector. If it changes, all 7,239 embeddings silently become stale —
// and nothing in the database can detect that, because a VECTOR(1024) column
// accepts a vector produced from any text by any model. This test is the tripwire,
// exactly like TestGoldenContentHash for the corpus hash.
func TestGoldenEmbedText(t *testing.T) {
	got := BuildEmbedText(
		"etcd v3.5 data inconsistency after crash",
		"Consistent index was not saved atomically.\nAn independent crash could lose committed transactions.",
	)
	want := "etcd v3.5 data inconsistency after crash\n\n" +
		"Consistent index was not saved atomically.\n" +
		"An independent crash could lose committed transactions."

	if got != want {
		t.Fatalf("embedding input spec changed\n  got:  %q\n  want: %q\n"+
			"If deliberate, every stored embedding is now stale and must be regenerated.", got, want)
	}
}

// An issue with no body must still embed its title rather than emitting a trailing
// separator that would differ from a body-less issue elsewhere.
func TestBuildEmbedTextEmptyBody(t *testing.T) {
	got := BuildEmbedText("just a title", "")
	if got != "just a title" {
		t.Fatalf("got %q, want %q", got, "just a title")
	}
	if strings.Contains(got, "\n\n") {
		t.Fatal("a body-less issue emitted a separator")
	}
}

// Truncation must never split a multi-byte character. A body cut mid-rune produces
// invalid UTF-8, which the model would reject or silently mangle — and etcd issues
// contain plenty of non-ASCII.
func TestBuildEmbedTextTruncatesOnRuneBoundary(t *testing.T) {
	// Three-byte runes guarantee the naive byte cut lands mid-character.
	body := strings.Repeat("日", MaxEmbedChars)
	got := BuildEmbedText("t", body)

	if len(got) > MaxEmbedChars {
		t.Fatalf("result is %d bytes, over the %d budget", len(got), MaxEmbedChars)
	}
	if !utf8.ValidString(got) {
		t.Fatal("truncation produced invalid UTF-8")
	}
}

// Truncation must be deterministic: the same issue always yields the same text,
// therefore the same vector.
func TestBuildEmbedTextIsDeterministic(t *testing.T) {
	title, body := "deterministic", strings.Repeat("payload ", 5000)
	if BuildEmbedText(title, body) != BuildEmbedText(title, body) {
		t.Fatal("BuildEmbedText is not deterministic")
	}
}

// The budget must actually bind, or the truncation policy is decorative.
func TestBuildEmbedTextRespectsBudget(t *testing.T) {
	got := BuildEmbedText("t", strings.Repeat("x", MaxEmbedChars*3))
	if len(got) > MaxEmbedChars {
		t.Fatalf("result is %d bytes, over the %d budget", len(got), MaxEmbedChars)
	}
	if len(got) < MaxEmbedChars-8 {
		t.Fatalf("result is %d bytes, suspiciously far under the %d budget", len(got), MaxEmbedChars)
	}
}

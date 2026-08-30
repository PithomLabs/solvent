package intent_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"

	"github.com/PithomLabs/solvent/internal/intent"
	"github.com/PithomLabs/solvent/internal/testdb"
	"github.com/PithomLabs/solvent/kernel"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// 002 carries the corpus layer. Applied everywhere so every environment has one
// schema shape; these packages do not use the corpus tables themselves.
var schemaPaths = []string{"../../db/001_schema.sql", "../../db/002_corpus.sql", "../../db/003_wizard.sql", "../../db/004_debt_vocabulary.sql"}

var (
	dsn    string
	shared *sql.DB
)

func TestMain(m *testing.M) {
	ctx := context.Background()
	dsn = testdb.DSN()

	name, _ := testdb.DBNameFromDSN(dsn)
	testdb.AcquireResetLock(name)

	if err := testdb.Reset(ctx, dsn, schemaPaths...); err != nil {
		fmt.Fprintf(os.Stderr, "intent tests cannot start: %v\n", err)
		testdb.ReleaseResetLock(name)
		os.Exit(1)
	}

	var err error
	shared, err = testdb.Open(dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "intent tests cannot start: open pool: %v\n", err)
		testdb.ReleaseResetLock(name)
		os.Exit(1)
	}
	if err := shared.PingContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "intent tests cannot start: ping: %v\n", err)
		testdb.ReleaseResetLock(name)
		os.Exit(1)
	}

	code := m.Run()
	_ = shared.Close()
	testdb.ReleaseResetLock(name)
	os.Exit(code)
}

func scenario(n int) string {
	return fmt.Sprintf("33333333-0000-0000-0000-%012x", n)
}

func createPromotedBelief(ctx context.Context, t *testing.T, sc, claim string) string {
	t.Helper()
	st := kernel.New(shared)
	id, err := st.EnsureBelief(ctx, sc, claim, kernel.Derived)
	if err != nil {
		t.Fatalf("EnsureBelief: %v", err)
	}
	for _, item := range kernel.FullDebt {
		if err := st.RetireDebt(ctx, id, item); err != nil {
			t.Fatalf("RetireDebt(%s): %v", item, err)
		}
	}
	if err := st.Promote(ctx, id); err != nil {
		t.Fatalf("Promote: %v", err)
	}
	return id
}

func TestPropose_OnPromotedBelief(t *testing.T) {
	ctx := context.Background()
	sc := scenario(1)
	beliefID := createPromotedBelief(ctx, t, sc, "etcd v3.5.x is vulnerable")

	err := intent.Propose(ctx, shared, sc, beliefID, "upgrade")
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}

	var count int
	_ = shared.QueryRowContext(ctx,
		`SELECT count(*) FROM action_intent WHERE scenario_id=$1::UUID AND state='live'`, sc).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 live intent, got %d", count)
	}
}

func TestPropose_OnUnpromotedBelief(t *testing.T) {
	ctx := context.Background()
	sc := scenario(2)
	st := kernel.New(shared)

	beliefID, err := st.EnsureBelief(ctx, sc, "unpromoted belief", kernel.Derived)
	if err != nil {
		t.Fatalf("EnsureBelief: %v", err)
	}

	err = intent.Propose(ctx, shared, sc, beliefID, "upgrade")
	if err == nil {
		t.Fatal("expected error for action on unpromoted belief, got nil")
	}

	var count int
	_ = shared.QueryRowContext(ctx,
		`SELECT count(*) FROM action_intent WHERE scenario_id=$1::UUID`, sc).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 intents, got %d", count)
	}
}

func TestAudit_NoLiveIntentsOnNonPromoted(t *testing.T) {
	ctx := context.Background()
	sc := scenario(3)
	createPromotedBelief(ctx, t, sc, "promoted belief")

	n, err := intent.Audit(ctx, shared, sc)
	if err != nil {
		t.Fatalf("Audit returned error: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 live intents on non-promoted, got %d", n)
	}
}

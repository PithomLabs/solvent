package api_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/PithomLabs/solvent/internal/testdb"
)

const testPrincipalID = "00000000-0000-0000-0000-000000000001"

var schemaPaths = []string{
	"../db/001_schema.sql",
	"../db/002_corpus.sql",
	"../db/003_wizard.sql",
	"../db/004_debt_vocabulary.sql",
	"../db/005_authority_mvp.sql",
	"../db/006_authority_justification_cascade.sql",
	"../db/007_service_tables.sql",
	"../db/008_executing_state.sql",
		"../db/009_exact_authority_binding.sql",
}

func TestMain(m *testing.M) {
	ctx := context.Background()
	dsn := testdb.SuiteDSN("api")

	name, _ := testdb.DBNameFromDSN(dsn)
	testdb.AcquireResetLock(name)

	if err := testdb.Reset(ctx, dsn, schemaPaths...); err != nil {
		fmt.Fprintf(os.Stderr, "api tests cannot start: %v\n", err)
		testdb.ReleaseResetLock(name)
		os.Exit(1)
	}

	// Insert the test principal used by AuthMiddleware so FK references succeed.
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "api tests cannot start: open: %v\n", err)
		testdb.ReleaseResetLock(name)
		os.Exit(1)
	}
	_, err = db.ExecContext(ctx,
		`INSERT INTO principal (principal_id, principal_type, issuer)
		 VALUES ($1::UUID, 'service', 'test-api')
		 ON CONFLICT (principal_id) DO NOTHING`,
		testPrincipalID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "api tests cannot start: insert test principal: %v\n", err)
		db.Close()
		testdb.ReleaseResetLock(name)
		os.Exit(1)
	}
	db.Close()

	code := m.Run()

	testdb.ReleaseResetLock(name)
	os.Exit(code)
}

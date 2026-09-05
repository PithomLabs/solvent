package api_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/PithomLabs/solvent/internal/testdb"
)

var schemaPaths = []string{
	"../db/001_schema.sql",
	"../db/002_corpus.sql",
	"../db/003_wizard.sql",
	"../db/004_debt_vocabulary.sql",
	"../db/005_authority_mvp.sql",
	"../db/006_authority_justification_cascade.sql",
	"../db/007_service_tables.sql",
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

	code := m.Run()

	testdb.ReleaseResetLock(name)
	os.Exit(code)
}

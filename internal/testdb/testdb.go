// Package testdb is the M2 behavioral suite's cluster harness.
//
// Verification support. Not part of the public kernel API, and never imported by
// internal/kernel's non-test files.
//
// It owns one dangerous operation — dropping and recreating the suite's database —
// so the guard that makes that safe lives here rather than in a caller.
package testdb

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

// DefaultDSN points at a database whose name ends in _test, which is what Reset
// requires. The M0/M1 database (`fable`) can never be reached by accident.
const DefaultDSN = "postgresql://root@localhost:26260/fable_test?sslmode=disable"

// requiredSuffix is the safety guard (M2-R1). Reset refuses any database whose name
// does not end in this.
const requiredSuffix = "_test"

// DSN returns the suite's connection string, overridable for a different cluster.
func DSN() string {
	if v := os.Getenv("FABLE_TEST_DSN"); v != "" {
		return v
	}
	return DefaultDSN
}

// DBName extracts the database name from a DSN.
func DBName(dsn string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", err
	}
	name := strings.TrimPrefix(u.Path, "/")
	if name == "" {
		return "", fmt.Errorf("DSN carries no database name")
	}
	return name, nil
}

// Redact strips the password so a DSN can be printed (N1).
func Redact(dsn string) string {
	u, err := url.Parse(dsn)
	if err != nil {
		return "<unparseable DSN>"
	}
	if u.User != nil {
		u.User = url.User(u.User.Username())
	}
	return u.String()
}

// WithOptions returns dsn with PostgreSQL session variables attached as connection
// parameters, e.g. WithOptions(dsn, map[string]string{"inject_retry_errors_enabled": "true"}).
//
// Connection parameters rather than SET: database/sql pools connections, so a SET on
// one connection is not necessarily the connection a later transaction uses. A DSN
// option applies to every connection the pool opens.
func WithOptions(dsn string, vars map[string]string, appName string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", err
	}
	q := u.Query()

	if len(vars) > 0 {
		// Deterministic order: map iteration is not stable and this string ends up
		// in a transcript.
		keys := make([]string, 0, len(vars))
		for k := range vars {
			keys = append(keys, k)
		}
		sortStrings(keys)

		var opts []string
		for _, k := range keys {
			opts = append(opts, fmt.Sprintf("-c %s=%s", k, vars[k]))
		}
		q.Set("options", strings.Join(opts, " "))
	}
	if appName != "" {
		q.Set("application_name", appName)
	}

	u.RawQuery = q.Encode()
	return u.String(), nil
}

// Open opens a pool. It does not ping.
func Open(dsn string) (*sql.DB, error) {
	return sql.Open("pgx", dsn)
}

// lockPath returns the path for a database reset lock file.
func lockPath(name string) string {
	return fmt.Sprintf("/tmp/%s.reset.lock", name)
}

// AcquireResetLock creates a lock file to prevent concurrent test suites from
// operating on the same database. Blocks until the lock is acquired. The caller
// MUST call ReleaseResetLock when the test suite is done.
func AcquireResetLock(name string) {
	path := lockPath(name)
	for {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err == nil {
			_ = f.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// ReleaseResetLock removes the lock file.
func ReleaseResetLock(name string) {
	_ = os.Remove(lockPath(name))
}

// DBNameFromDSN extracts the database name for use with AcquireResetLock/ReleaseResetLock.
func DBNameFromDSN(dsn string) (string, error) {
	return DBName(dsn)
}

// Reset drops and recreates the suite's database, then applies the frozen DDL.
//
// GUARD (M2-R1): it refuses outright unless the database name ends in "_test". This
// function destroys data; a mistyped DSN pointing at `fable`, or at a shared cluster,
// would otherwise take the M0 and M1 evidence with it. The refusal is an error, not a
// warning.
//
// The caller MUST hold AcquireResetLock before calling Reset, and MUST hold it through
// the entire test suite (m.Run) before calling ReleaseResetLock.
func Reset(ctx context.Context, dsn, schemaPath string) error {
	name, err := DBName(dsn)
	if err != nil {
		return err
	}
	if !strings.HasSuffix(name, requiredSuffix) {
		return fmt.Errorf(
			"refusing to reset database %q: the M2 suite drops its database, so the name must end in %q (got DSN %s)",
			name, requiredSuffix, Redact(dsn))
	}

	// N1: say what is about to be destroyed, before destroying it.
	fmt.Printf("=== Wave 0 === resetting behavioral test database\n")
	fmt.Printf("    dsn:      %s\n", Redact(dsn))
	fmt.Printf("    database: %s  (DROP + CREATE + apply %s)\n", name, schemaPath)

	admin := dsn
	if u, err := url.Parse(dsn); err == nil {
		u.Path = "/defaultdb"
		admin = u.String()
	}

	adminDB, err := Open(admin)
	if err != nil {
		return err
	}
	defer func() { _ = adminDB.Close() }()

	if _, err := adminDB.ExecContext(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %q CASCADE", name)); err != nil {
		return fmt.Errorf("drop %s: %w", name, err)
	}
	if _, err := adminDB.ExecContext(ctx, fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %q", name)); err != nil {
		return fmt.Errorf("create %s: %w", name, err)
	}

	return ApplySchema(ctx, dsn, schemaPath)
}

// ApplySchema applies the frozen DDL statement by statement.
func ApplySchema(ctx context.Context, dsn, schemaPath string) error {
	raw, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", schemaPath, err)
	}

	db, err := Open(dsn)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	for _, stmt := range splitStatements(string(raw)) {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("apply schema: %w\nstatement: %s", err, stmt)
		}
	}
	return nil
}

// splitStatements strips line comments and splits on semicolons. Exact for
// db/001_schema.sql, which contains no dollar-quoting and no string literal holding a
// semicolon; deliberately not a general SQL parser.
//
// Duplicated from internal/m0's applier rather than imported: internal/m0 is frozen at
// M0's close, and internal/kernel must be able to depend on this package without any
// path back to it.
func splitStatements(sqlText string) []string {
	var cleaned strings.Builder
	for _, line := range strings.Split(sqlText, "\n") {
		if idx := strings.Index(line, "--"); idx >= 0 {
			line = line[:idx]
		}
		cleaned.WriteString(line)
		cleaned.WriteString("\n")
	}

	var out []string
	for _, part := range strings.Split(cleaned.String(), ";") {
		if s := strings.TrimSpace(part); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

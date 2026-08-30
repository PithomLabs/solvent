package main

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// toolError maps a kernel error to a map suitable for an MCP tool result.
//
// If err carries a *pgconn.PgError (reachable through the kernel's sentinel
// wraps), the sqlstate and constraint name are included verbatim. Otherwise
// a plain message is returned.
func toolError(err error) map[string]interface{} {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return map[string]interface{}{
			"error":      true,
			"sentinel":   err.Error(),
			"sqlstate":   pgErr.SQLState(),
			"constraint": pgErr.ConstraintName,
		}
	}
	return map[string]interface{}{
		"error":   true,
		"message": err.Error(),
	}
}

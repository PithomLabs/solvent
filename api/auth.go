package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/PithomLabs/solvent/kernel"
)

// AuthenticatedPrincipal represents the trusted identity from the API key.
type AuthenticatedPrincipal struct {
	PrincipalID   string
	PrincipalType string
	AuthMethod    string
	AuthTime      time.Time
}

type contextKey string

const authKey contextKey = "authenticated_principal"

// AuthMiddleware validates Bearer tokens against keyToPrincipal.
// Each API key maps to exactly one Solvent principal_id. The mapping is
// static configuration — it does not create, mutate, or revoke principals.
// Sets AuthenticatedPrincipal on context for downstream handlers.
func AuthMiddleware(keyToPrincipal map[string]string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			writeError(w, http.StatusUnauthorized, "missing_authorization", "Authorization header required", nil)
			return
		}

		if !strings.HasPrefix(auth, "Bearer ") {
			writeError(w, http.StatusUnauthorized, "invalid_authorization", "Authorization header must use Bearer scheme", nil)
			return
		}

		token := strings.TrimPrefix(auth, "Bearer ")
		principalID, ok := keyToPrincipal[token]
		if !ok {
			writeError(w, http.StatusUnauthorized, "invalid_token", "Invalid API key", nil)
			return
		}

		principal := &AuthenticatedPrincipal{
			PrincipalID:   principalID,
			PrincipalType: "service",
			AuthMethod:    "api-key",
			AuthTime:      time.Now(),
		}
		ctx := context.WithValue(r.Context(), authKey, principal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AuthFromContext extracts the authenticated principal from request context.
func AuthFromContext(ctx context.Context) *AuthenticatedPrincipal {
	p, _ := ctx.Value(authKey).(*AuthenticatedPrincipal)
	return p
}

// verifyPrincipalActive checks that the given principal exists in the principal
// table and has not been revoked. Returns nil if active, kernel.ErrRevokedPrincipal
// if revoked, or a generic error if not found.
//
// This is a best-effort liveness check, not a transactionally atomic authorization
// gate. The kernel owns its internal crdb.ExecuteTx, so this check cannot be made
// atomic with downstream kernel mutations. The residual TOCTOU race is documented
// and accepted: revocation takes effect immediately for all new requests.
func verifyPrincipalActive(ctx context.Context, db *sql.DB, principalID string) error {
	var revokedAt sql.NullTime
	err := db.QueryRowContext(ctx,
		`SELECT revoked_at FROM principal WHERE principal_id = $1::UUID`,
		principalID).Scan(&revokedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return kernel.ErrRevokedPrincipal
	}
	if err != nil {
		return err
	}
	if revokedAt.Valid {
		return kernel.ErrRevokedPrincipal
	}
	return nil
}

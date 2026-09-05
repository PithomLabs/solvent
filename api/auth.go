package api

import (
	"context"
	"net/http"
	"strings"
	"time"
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

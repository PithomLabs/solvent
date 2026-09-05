package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/PithomLabs/solvent/api"
	"github.com/PithomLabs/solvent/service/audit"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	dbURL := os.Getenv("SOLVENT_DATABASE_URL")
	if dbURL == "" {
		log.Fatal("SOLVENT_DATABASE_URL not set")
	}

	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("database ping: %v", err)
	}

	// Parse SOLVENT_API_KEYS="key1=<principal-uuid>,key2=<principal-uuid>"
	// A configured API key maps to exactly one Solvent principal_id.
	// Configuration does not create, mutate, revoke, or otherwise manage principals.
	rawKeys := os.Getenv("SOLVENT_API_KEYS")
	if rawKeys == "" {
		log.Fatal("SOLVENT_API_KEYS not set or empty")
	}
	keyToPrincipal := make(map[string]string)
	for _, entry := range strings.Split(rawKeys, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			log.Fatalf("SOLVENT_API_KEYS: malformed entry %q (expected key=<principal-uuid>)", entry)
		}
		keyToPrincipal[parts[0]] = parts[1]
	}
	if len(keyToPrincipal) == 0 {
		log.Fatal("SOLVENT_API_KEYS: no valid entries")
	}

	auditSvc := audit.New(db)
	server := api.NewServer(db, auditSvc)

	handler := api.AuthMiddleware(keyToPrincipal, server.Handler())

	addr := ":8080"
	if a := os.Getenv("SOLVENT_API_ADDR"); a != "" {
		addr = a
	}

	fmt.Printf("solvent-api listening on %s (%d principals configured)\n", addr, len(keyToPrincipal))
	log.Fatal(http.ListenAndServe(addr, handler))
}

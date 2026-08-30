// Command solvent-web is the hosted judge-facing Solvent demo.
//
// Two surfaces, deliberately separate:
//
//   - `/` and its ledger pages are read-only. They read from CockroachDB and serve the
//     live Track 2 ledger. No writes, no feeds, no authentication.
//   - `/demo` is the three-screen decision wizard (internal/wizard), which DOES write:
//     it seeds its own scenario per visitor and drives the kernel's promote, authorize
//     and debt-retirement paths so a judge can watch the database refuse and accept.
//
// The wizard is mounted under a prefix rather than at `/` so the recorded live URL
// keeps working unchanged. Promoting it to `/` is a later phase, gated on the App
// Runner service having an instance role that permits bedrock:InvokeModel — without
// which the wizard's search cannot run at all.
package main

import (
	"context"
	"database/sql"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/PithomLabs/solvent/internal/wizard"
	_ "github.com/jackc/pgx/v5/stdlib"
)

var (
	db        *sql.DB
	templates *template.Template
)

func main() {
	dsn := os.Getenv("FABLE_DSN")
	if dsn == "" {
		log.Fatal("FABLE_DSN not set")
	}

	var err error
	db, err = sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("ping db: %v", err)
	}

	templates, err = template.New("").Funcs(templateFuncs).ParseGlob("demo/cloud/web/templates/*.html")
	if err != nil {
		log.Fatalf("parse templates: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/ledger", handleLedger)
	mux.HandleFunc("/beliefs", handleBeliefs)
	mux.HandleFunc("/belief/", handleBeliefDetail)
	mux.HandleFunc("/evidence", handleEvidence)
	mux.HandleFunc("/intents", handleIntents)
	mux.HandleFunc("/audit", handleAudit)
	mux.HandleFunc("/health", handleHealth)

	// The wizard owns everything under its own prefix and nothing outside it. It fails
	// soft: if it cannot be constructed the ledger pages still serve, because a judge
	// arriving at the recorded URL should never meet a dead site.
	wiz, err := wizard.New(db, wizard.Options{})
	if err != nil {
		log.Printf("wizard unavailable, serving ledger only: %v", err)
	} else {
		wiz.Routes(mux)
		log.Printf("wizard mounted at %s", wizard.Prefix)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:        ":" + port,
		Handler:     loggingMiddleware(mux),
		ReadTimeout: 5 * time.Second,
		// 20s, up from 10s. The wizard's search embeds the judge's query with Bedrock
		// before it can run the ANN query, and a cold serverless cluster adds to that.
		// The search handler bounds itself to 8s independently, so this is headroom
		// rather than a licence to hang.
		WriteTimeout: 20 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("solvent-web listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down...")
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("shutdown: %v", err)
	}
}

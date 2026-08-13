// Command solvent-web is the hosted judge-facing read-only Solvent ledger.
//
// It reads from CockroachDB and serves the live ledger state over HTTPS.
// No write operations are exposed. No external feeds. No authentication.
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

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      loggingMiddleware(mux),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
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

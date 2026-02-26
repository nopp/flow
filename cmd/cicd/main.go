// Command cicd is the entry point for the PiaFlow server.
// It loads apps from YAML, opens the SQLite store, and starts the HTTP server
// that serves the web UI and the REST API for apps and runs.
package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"piaflow/internal/config"
	"piaflow/internal/pipeline"
	"piaflow/internal/server"
	"piaflow/internal/store"
)

func main() {
	configPath := flag.String("config", "config/apps.yaml", "path to apps.yaml")
	dbPath := flag.String("db", "data/cicd.db", "path to SQLite database")
	workDir := flag.String("work", "work", "directory for cloning repos")
	staticDir := flag.String("static", "web", "directory for web UI static files")
	addr := flag.String("addr", ":8080", "HTTP listen address")
	flag.Parse()

	if err := os.MkdirAll(filepath.Dir(*dbPath), 0755); err != nil {
		log.Fatalf("create data dir: %v", err)
	}
	if err := os.MkdirAll(*workDir, 0755); err != nil {
		log.Fatalf("create work dir: %v", err)
	}

	apps, err := config.LoadApps(*configPath)
	if err != nil {
		log.Fatalf("load apps config: %v", err)
	}

	st, err := store.New(*dbPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()

	runner := pipeline.NewRunner(*workDir)
	absConfig, _ := filepath.Abs(*configPath)
	staticPath, _ := filepath.Abs(*staticDir)
	srv := server.New(apps, st, runner, absConfig, staticPath)

	log.Printf("listening on %s", *addr)
	if err := http.ListenAndServe(*addr, srv.Handler()); err != nil {
		log.Fatalf("server: %v", err)
	}
}

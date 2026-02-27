// Command cicd is the entry point for the NoppFlow server.
// It loads apps from YAML, opens the store (SQLite or MySQL), and starts the HTTP server
// that serves the web UI and the REST API for apps and runs.
package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"noppflow/internal/auth"
	"noppflow/internal/config"
	"noppflow/internal/pipeline"
	"noppflow/internal/server"
	"noppflow/internal/store"
)

func main() {
	configPath := flag.String("config", "config/apps.yaml", "path to apps.yaml")
	dbPath := flag.String("db", "data/cicd.db", "path to SQLite database (used when DB_DRIVER is not mysql)")
	workDir := flag.String("work", "work", "directory for cloning repos")
	staticDir := flag.String("static", "web", "directory for web UI static files")
	addr := flag.String("addr", ":8080", "HTTP listen address")
	flag.Parse()

	dbDriver := strings.TrimSpace(os.Getenv("DB_DRIVER"))
	if dbDriver == "" {
		dbDriver = "sqlite3"
	}

	var dbDSN string
	switch dbDriver {
	case "mysql":
		dbDSN = strings.TrimSpace(os.Getenv("DB_DSN"))
		if dbDSN == "" {
			log.Fatal("DB_DSN is required when DB_DRIVER=mysql (e.g. user:password@tcp(host:3306)/dbname?parseTime=true)")
		}
	default:
		dbDriver = "sqlite3"
		dbDSN = *dbPath
		if err := os.MkdirAll(filepath.Dir(dbDSN), 0755); err != nil {
			log.Fatalf("create data dir: %v", err)
		}
	}

	if err := os.MkdirAll(*workDir, 0755); err != nil {
		log.Fatalf("create work dir: %v", err)
	}

	apps, err := config.LoadApps(*configPath)
	if err != nil {
		log.Fatalf("load apps config: %v", err)
	}

	st, err := store.New(dbDriver, dbDSN)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()

	adminUsername := strings.TrimSpace(os.Getenv("ADMIN_USERNAME"))
	if adminUsername == "" {
		adminUsername = "admin"
	}
	adminPassword := strings.TrimSpace(os.Getenv("ADMIN_PASSWORD"))
	if adminPassword == "" {
		adminPassword = "admin"
	}
	adminHash, err := auth.HashPassword(adminPassword)
	if err != nil {
		log.Fatalf("hash admin password: %v", err)
	}
	if err := st.EnsureAdminUser(adminUsername, adminHash); err != nil {
		log.Fatalf("ensure admin user: %v", err)
	}

	runner := pipeline.NewRunner(*workDir)
	absConfig, _ := filepath.Abs(*configPath)
	staticPath, _ := filepath.Abs(*staticDir)
	srv := server.New(apps, st, runner, absConfig, staticPath)

	log.Printf("listening on %s", *addr)
	if err := http.ListenAndServe(*addr, srv.Handler()); err != nil {
		log.Fatalf("server: %v", err)
	}
}

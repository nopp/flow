// Package server provides the HTTP API and serves the web UI.
// It uses Chi for routing, protects the apps slice with a RWMutex, and delegates
// run execution to the pipeline Runner and persistence to the Store.
package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"piaflow/internal/config"
	"piaflow/internal/pipeline"
	"piaflow/internal/store"
)

// Server holds the in-memory app list, store, pipeline runner, and paths for config and static files.
// appsMu protects reads and writes to apps; config changes are persisted via config.SaveApps.
type Server struct {
	appsMu    sync.RWMutex
	apps      []config.App
	store     *store.Store
	runner    *pipeline.Runner
	appsPath  string
	staticDir string
}

// New builds a Server with the given apps slice, store, runner, and absolute paths to config and static dir.
func New(apps []config.App, st *store.Store, runner *pipeline.Runner, appsPath, staticDir string) *Server {
	return &Server{apps: apps, store: st, runner: runner, appsPath: appsPath, staticDir: staticDir}
}

// Handler returns the Chi router: /health, /api/apps, /api/apps/:id, /api/apps/:id/run, /api/runs, /api/runs/:id, and /* for static files.
func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", s.health)
	r.Route("/api", func(r chi.Router) {
		r.Get("/apps", s.listApps)
		r.Post("/apps", s.createApp)
		r.Get("/apps/{appID}", s.getApp)
		r.Put("/apps/{appID}", s.updateApp)
		r.Delete("/apps/{appID}", s.deleteApp)
		r.Post("/apps/{appID}/run", s.triggerRun)
		r.Get("/runs", s.listRuns)
		r.Get("/runs/{id}", s.getRun)
	})
	r.Get("/*", s.serveStatic)
	return r
}

// serveStatic serves the web UI; falls back to index.html for SPA routes.
func (s *Server) serveStatic(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" || r.URL.Path == "" {
		http.ServeFile(w, r, filepath.Join(s.staticDir, "index.html"))
		return
	}
	path := filepath.Join(s.staticDir, strings.TrimPrefix(r.URL.Path, "/"))
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		http.ServeFile(w, r, path)
		return
	}
	http.ServeFile(w, r, filepath.Join(s.staticDir, "index.html"))
}

// health responds with 200 "ok" for readiness checks.
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// listApps returns all apps as JSON [{id, name}].
func (s *Server) listApps(w http.ResponseWriter, r *http.Request) {
	s.appsMu.RLock()
	defer s.appsMu.RUnlock()
	type app struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	out := make([]app, len(s.apps))
	for i := range s.apps {
		out[i] = app{ID: s.apps[i].ID, Name: s.apps[i].Name}
	}
	writeJSON(w, http.StatusOK, out)
}

// getApp returns one app by appID (full fields) or 404.
func (s *Server) getApp(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "appID")
	s.appsMu.RLock()
	defer s.appsMu.RUnlock()
	for _, a := range s.apps {
		if a.ID == appID {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"id": a.ID, "name": a.Name, "repo": a.Repo, "branch": a.Branch,
				"test_cmd": a.TestCmd, "build_cmd": a.BuildCmd, "deploy_cmd": a.DeployCmd,
				"test_sleep_sec": a.TestSleepSec, "build_sleep_sec": a.BuildSleepSec, "deploy_sleep_sec": a.DeploySleepSec,
			})
			return
		}
	}
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "app not found"})
}

// createApp decodes JSON body, validates, appends to apps, saves YAML, returns 201 or 4xx/5xx.
func (s *Server) createApp(w http.ResponseWriter, r *http.Request) {
	var body struct {
		config.App
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	app := body.App
	if app.ID == "" || app.Name == "" || app.Repo == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id, name and repo are required"})
		return
	}
	if strings.TrimSpace(app.TestCmd) == "" || strings.TrimSpace(app.BuildCmd) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "test_cmd and build_cmd are required"})
		return
	}
	if app.Branch == "" {
		app.Branch = "main"
	}
	if app.TestSleepSec < 0 || app.BuildSleepSec < 0 || app.DeploySleepSec < 0 ||
		app.TestSleepSec > 3600 || app.BuildSleepSec > 3600 || app.DeploySleepSec > 3600 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "test_sleep_sec, build_sleep_sec, deploy_sleep_sec must be between 0 and 3600"})
		return
	}
	s.appsMu.Lock()
	defer s.appsMu.Unlock()
	for _, a := range s.apps {
		if a.ID == app.ID {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "app id already exists"})
			return
		}
	}
	newApps := append(s.apps, app)
	if err := config.SaveApps(s.appsPath, newApps); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.apps = newApps
	writeJSON(w, http.StatusCreated, app)
}

// updateApp decodes JSON body, finds app by appID, updates, saves YAML, returns 200 or 4xx/5xx.
func (s *Server) updateApp(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "appID")
	var body struct {
		config.App
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	app := body.App
	app.ID = appID
	if strings.TrimSpace(app.TestCmd) == "" || strings.TrimSpace(app.BuildCmd) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "test_cmd and build_cmd are required"})
		return
	}
	if app.Branch == "" {
		app.Branch = "main"
	}
	if app.TestSleepSec < 0 || app.BuildSleepSec < 0 || app.DeploySleepSec < 0 ||
		app.TestSleepSec > 3600 || app.BuildSleepSec > 3600 || app.DeploySleepSec > 3600 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "test_sleep_sec, build_sleep_sec, deploy_sleep_sec must be between 0 and 3600"})
		return
	}
	s.appsMu.Lock()
	defer s.appsMu.Unlock()
	found := false
	for i := range s.apps {
		if s.apps[i].ID == appID {
			s.apps[i] = app
			found = true
			break
		}
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "app not found"})
		return
	}
	if err := config.SaveApps(s.appsPath, s.apps); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, app)
}

// deleteApp removes the app from the slice, saves YAML, returns 204 or 404.
func (s *Server) deleteApp(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "appID")
	s.appsMu.Lock()
	defer s.appsMu.Unlock()
	var newApps []config.App
	for _, a := range s.apps {
		if a.ID != appID {
			newApps = append(newApps, a)
		}
	}
	if len(newApps) == len(s.apps) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "app not found"})
		return
	}
	if err := config.SaveApps(s.appsPath, newApps); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.apps = newApps
	w.WriteHeader(http.StatusNoContent)
}

// triggerRun creates a run in the store, starts the pipeline in a goroutine, returns 202 {run_id, status}.
func (s *Server) triggerRun(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "appID")
	s.appsMu.RLock()
	var app *config.App
	for i := range s.apps {
		if s.apps[i].ID == appID {
			app = &s.apps[i]
			break
		}
	}
	s.appsMu.RUnlock()
	if app == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "app not found"})
		return
	}

	runID, err := s.store.CreateRun(appID, "")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	appCopy := *app
	go func() {
		_ = s.store.UpdateRunStatus(runID, "running", "")
		onLogUpdate := func(log string) { _ = s.store.UpdateRunLog(runID, log) }
		result := s.runner.Run(appCopy, onLogUpdate)
		status := "success"
		if !result.Success {
			status = "failed"
		}
		_ = s.store.UpdateRunStatus(runID, status, result.Log)
	}()

	writeJSON(w, http.StatusAccepted, map[string]interface{}{"run_id": runID, "status": "pending"})
}

// listRuns returns runs (optional app_id and limit query params) as JSON.
func (s *Server) listRuns(w http.ResponseWriter, r *http.Request) {
	appID := r.URL.Query().Get("app_id")
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 {
			limit = n
		}
	}
	runs, err := s.store.ListRuns(appID, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, runs)
}

// getRun returns one run by ID or 404.
func (s *Server) getRun(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid run id"})
		return
	}
	run, err := s.store.GetRun(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if run == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "run not found"})
		return
	}
	writeJSON(w, http.StatusOK, run)
}

// writeJSON sets Content-Type application/json, status code, and encodes v as JSON.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

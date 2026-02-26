// Package server provides the HTTP API and serves the web UI.
package server

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"piaflow/internal/auth"
	"piaflow/internal/config"
	"piaflow/internal/pipeline"
	"piaflow/internal/store"
)

const sessionCookieName = "piaflow_session"
const (
	sessionTTLSeconds      = 30 * 60 // 30 minutes
	sessionRotateThreshold = 10 * time.Minute
)

type contextKey string

const authUserKey contextKey = "auth_user"

type authUser struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	IsAdmin  bool   `json:"is_admin"`
}

type sessionData struct {
	User      authUser
	ExpiresAt time.Time
}

// Server holds app data, store, runner and session state.
type Server struct {
	appsMu    sync.RWMutex
	apps      []config.App
	store     *store.Store
	runner    *pipeline.Runner
	appsPath  string
	staticDir string

	sessionsMu sync.RWMutex
	sessions   map[string]sessionData
}

// New builds a Server with the given apps slice, store, runner, and paths.
func New(apps []config.App, st *store.Store, runner *pipeline.Runner, appsPath, staticDir string) *Server {
	return &Server{
		apps:      apps,
		store:     st,
		runner:    runner,
		appsPath:  appsPath,
		staticDir: staticDir,
		sessions:  make(map[string]sessionData),
	}
}

// Handler returns the router for API and static pages.
func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", s.health)
	r.Route("/api", func(r chi.Router) {
		r.Post("/auth/login", s.login)
		r.Post("/auth/logout", s.logout)
		r.Get("/auth/me", s.me)

		r.Group(func(r chi.Router) {
			r.Use(s.requireAuth)
			r.Put("/auth/password", s.changeMyPassword)
			r.Get("/auth/profile", s.profile)
			r.Get("/users", s.listUsers)
			r.Post("/users", s.createUser)
			r.Put("/users/{userID}/groups", s.setUserGroups)
			r.Put("/users/{userID}/password", s.updateUserPassword)
			r.Delete("/users/{userID}", s.deleteUser)
			r.Get("/groups", s.listGroups)
			r.Post("/groups", s.createGroup)
			r.Get("/groups/{groupID}", s.getGroup)
			r.Put("/groups/{groupID}/users", s.setGroupUsers)
			r.Put("/groups/{groupID}/apps", s.setGroupApps)
			r.Get("/apps", s.listApps)
			r.Post("/apps", s.createApp)
			r.Get("/apps/{appID}", s.getApp)
			r.Put("/apps/{appID}", s.updateApp)
			r.Delete("/apps/{appID}", s.deleteApp)
			r.Get("/apps/{appID}/groups", s.getAppGroups)
			r.Put("/apps/{appID}/groups", s.setAppGroups)
			r.Post("/apps/{appID}/run", s.triggerRun)
			r.Get("/runs", s.listRuns)
			r.Get("/runs/{id}", s.getRun)
		})
	})
	r.Get("/*", s.serveStatic)
	return r
}

func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, _, ok := s.authenticateSession(w, r, true)
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
			return
		}
		ctx := context.WithValue(r.Context(), authUserKey, u)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func authUserFromContext(r *http.Request) authUser {
	v := r.Context().Value(authUserKey)
	if v == nil {
		return authUser{}
	}
	u, _ := v.(authUser)
	return u
}

func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) (authUser, bool) {
	u := authUserFromContext(r)
	if !u.IsAdmin {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "admin access required"})
		return authUser{}, false
	}
	return u, true
}

// serveStatic serves static files; falls back to index.html for unknown routes.
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

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	username := strings.TrimSpace(body.Username)
	password := strings.TrimSpace(body.Password)
	if username == "" || password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "username and password are required"})
		return
	}
	user, err := s.store.GetUserByUsername(username)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if user == nil || !auth.CheckPassword(password, user.PasswordHash) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}
	sessionUser := authUser{ID: user.ID, Username: user.Username, IsAdmin: user.IsAdmin}
	if err := s.createSession(w, sessionUser); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create session"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"user": sessionUser})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err == nil && cookie.Value != "" {
		s.sessionsMu.Lock()
		delete(s.sessions, cookie.Value)
		s.sessionsMu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	u, _, ok := s.authenticateSession(w, r, true)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"user": u})
}

func (s *Server) changeMyPassword(w http.ResponseWriter, r *http.Request) {
	user := authUserFromContext(r)
	var body struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	currentPassword := strings.TrimSpace(body.CurrentPassword)
	newPassword := strings.TrimSpace(body.NewPassword)
	if currentPassword == "" || newPassword == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "current_password and new_password are required"})
		return
	}
	dbUser, err := s.store.GetUser(user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if dbUser == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	}
	if !auth.CheckPassword(currentPassword, dbUser.PasswordHash) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid current password"})
		return
	}
	hash, err := auth.HashPassword(newPassword)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to hash password"})
		return
	}
	if err := s.store.UpdateUserPassword(user.ID, hash); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	// Invalidate all existing sessions for this user, then create a fresh session.
	s.invalidateUserSessions(user.ID)
	if err := s.createSession(w, user); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to refresh session"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"password_updated": true})
}

func (s *Server) profile(w http.ResponseWriter, r *http.Request) {
	user := authUserFromContext(r)
	dbUser, err := s.store.GetUser(user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if dbUser == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	}

	groupMap := make(map[int64]struct{}, len(dbUser.GroupIDs))
	for _, gid := range dbUser.GroupIDs {
		groupMap[gid] = struct{}{}
	}
	allGroups, err := s.store.ListGroups()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	type groupOut struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}
	groupsOut := make([]groupOut, 0, len(dbUser.GroupIDs))
	for _, g := range allGroups {
		if _, ok := groupMap[g.ID]; ok {
			groupsOut = append(groupsOut, groupOut{ID: g.ID, Name: g.Name})
		}
	}

	s.appsMu.RLock()
	type appOut struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Repo string `json:"repo"`
	}
	appsOut := make([]appOut, 0)
	if user.IsAdmin {
		appsOut = make([]appOut, 0, len(s.apps))
		for _, a := range s.apps {
			appsOut = append(appsOut, appOut{ID: a.ID, Name: a.Name, Repo: a.Repo})
		}
	} else {
		allowed, _, err := s.allowedAppIDsForUser(user.ID)
		if err != nil {
			s.appsMu.RUnlock()
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		for _, a := range s.apps {
			if _, ok := allowed[a.ID]; !ok {
				continue
			}
			appsOut = append(appsOut, appOut{ID: a.ID, Name: a.Name, Repo: a.Repo})
		}
	}
	s.appsMu.RUnlock()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":       user.ID,
		"username": user.Username,
		"is_admin": user.IsAdmin,
		"groups":   groupsOut,
		"apps":     appsOut,
	})
}

// listApps returns all apps for admin, or only allowed apps for normal users.
func (s *Server) listApps(w http.ResponseWriter, r *http.Request) {
	user := authUserFromContext(r)
	allowed := map[string]struct{}(nil)
	if !user.IsAdmin {
		var err error
		allowed, _, err = s.allowedAppIDsForUser(user.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	s.appsMu.RLock()
	defer s.appsMu.RUnlock()
	type app struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	out := make([]app, 0, len(s.apps))
	for i := range s.apps {
		if !user.IsAdmin {
			if _, ok := allowed[s.apps[i].ID]; !ok {
				continue
			}
		}
		out = append(out, app{ID: s.apps[i].ID, Name: s.apps[i].Name})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) getApp(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "appID")
	user := authUserFromContext(r)
	if !user.IsAdmin {
		ok, err := s.userCanAccessApp(user.ID, appID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if !ok {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "user has no access to this app"})
			return
		}
	}
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

func (s *Server) createApp(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
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

func (s *Server) updateApp(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "appID")
	user := authUserFromContext(r)
	if !user.IsAdmin {
		ok, err := s.userCanAccessApp(user.ID, appID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if !ok {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "user has no access to this app"})
			return
		}
	}
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

func (s *Server) deleteApp(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
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
	if err := s.store.DeleteRunsByAppID(appID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.apps = newApps
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) triggerRun(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "appID")
	user := authUserFromContext(r)
	if !user.IsAdmin {
		ok, err := s.userCanAccessApp(user.ID, appID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if !ok {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "user has no access to this app"})
			return
		}
	}

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

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	users, err := s.store.ListUsers()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	type userOut struct {
		ID       int64   `json:"id"`
		Username string  `json:"username"`
		GroupIDs []int64 `json:"group_ids"`
		IsAdmin  bool    `json:"is_admin"`
	}
	out := make([]userOut, 0, len(users))
	for _, u := range users {
		out = append(out, userOut{ID: u.ID, Username: u.Username, GroupIDs: u.GroupIDs, IsAdmin: u.IsAdmin})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var body struct {
		Username     string  `json:"username"`
		Password     string  `json:"password"`
		PasswordHash string  `json:"password_hash"`
		GroupIDs     []int64 `json:"group_ids"`
		IsAdmin      bool    `json:"is_admin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	body.Username = strings.TrimSpace(body.Username)
	body.Password = strings.TrimSpace(body.Password)
	body.PasswordHash = strings.TrimSpace(body.PasswordHash)
	if body.Username == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "username is required"})
		return
	}
	passwordHash := body.PasswordHash
	if body.Password != "" {
		var err error
		passwordHash, err = auth.HashPassword(body.Password)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to hash password"})
			return
		}
	}
	if passwordHash == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "password or password_hash is required"})
		return
	}
	id, err := s.store.CreateUser(body.Username, passwordHash, body.IsAdmin)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := s.store.SetUserGroups(id, body.GroupIDs); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id": id, "username": body.Username, "group_ids": body.GroupIDs, "is_admin": body.IsAdmin,
	})
}

func (s *Server) setUserGroups(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	userID, err := strconv.ParseInt(chi.URLParam(r, "userID"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user id"})
		return
	}
	var body struct {
		GroupIDs []int64 `json:"group_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	user, err := s.store.GetUser(userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if user == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	}
	if err := s.store.SetUserGroups(userID, body.GroupIDs); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"user_id": userID, "group_ids": body.GroupIDs})
}

func (s *Server) updateUserPassword(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	userID, err := strconv.ParseInt(chi.URLParam(r, "userID"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user id"})
		return
	}
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	password := strings.TrimSpace(body.Password)
	if password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "password is required"})
		return
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to hash password"})
		return
	}
	err = s.store.UpdateUserPassword(userID, hash)
	if storeErrNoRows(err) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"user_id": userID, "password_updated": true})
}

func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	userID, err := strconv.ParseInt(chi.URLParam(r, "userID"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user id"})
		return
	}
	if userID == admin.ID {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cannot delete current admin user"})
		return
	}
	target, err := s.store.GetUser(userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if target == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	}
	if target.IsAdmin {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cannot delete admin user"})
		return
	}
	err = s.store.DeleteUser(userID)
	if storeErrNoRows(err) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listGroups(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	groups, err := s.store.ListGroups()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, groups)
}

func (s *Server) createGroup(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	id, err := s.store.CreateGroup(body.Name)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{"id": id, "name": body.Name})
}

func (s *Server) getGroup(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	groupID, err := strconv.ParseInt(chi.URLParam(r, "groupID"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid group id"})
		return
	}
	group, err := s.store.GetGroup(groupID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if group == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "group not found"})
		return
	}
	userIDs, err := s.store.GroupUserIDs(groupID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	appIDs, err := s.store.GroupAppIDs(groupID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	users, err := s.store.ListUsers()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	type userOut struct {
		ID       int64  `json:"id"`
		Username string `json:"username"`
	}
	usersOut := make([]userOut, 0, len(users))
	for _, u := range users {
		usersOut = append(usersOut, userOut{ID: u.ID, Username: u.Username})
	}

	s.appsMu.RLock()
	type appOut struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	appsOut := make([]appOut, 0, len(s.apps))
	for _, a := range s.apps {
		appsOut = append(appsOut, appOut{ID: a.ID, Name: a.Name})
	}
	s.appsMu.RUnlock()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":              group.ID,
		"name":            group.Name,
		"user_ids":        userIDs,
		"app_ids":         appIDs,
		"available_users": usersOut,
		"available_apps":  appsOut,
	})
}

func (s *Server) setGroupUsers(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	groupID, err := strconv.ParseInt(chi.URLParam(r, "groupID"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid group id"})
		return
	}
	group, err := s.store.GetGroup(groupID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if group == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "group not found"})
		return
	}
	var body struct {
		UserIDs []int64 `json:"user_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if err := s.store.SetGroupUsers(groupID, body.UserIDs); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"group_id": groupID, "user_ids": body.UserIDs})
}

func (s *Server) setGroupApps(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	groupID, err := strconv.ParseInt(chi.URLParam(r, "groupID"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid group id"})
		return
	}
	group, err := s.store.GetGroup(groupID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if group == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "group not found"})
		return
	}
	var body struct {
		AppIDs []string `json:"app_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if err := s.store.SetGroupApps(groupID, body.AppIDs); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"group_id": groupID, "app_ids": body.AppIDs})
}

func (s *Server) getAppGroups(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	appID := chi.URLParam(r, "appID")
	if !s.appExists(appID) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "app not found"})
		return
	}
	groupIDs, err := s.store.AppGroupIDs(appID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"app_id": appID, "group_ids": groupIDs})
}

func (s *Server) setAppGroups(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	appID := chi.URLParam(r, "appID")
	if !s.appExists(appID) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "app not found"})
		return
	}
	var body struct {
		GroupIDs []int64 `json:"group_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if err := s.store.SetAppGroups(appID, body.GroupIDs); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"app_id": appID, "group_ids": body.GroupIDs})
}

func (s *Server) listRuns(w http.ResponseWriter, r *http.Request) {
	user := authUserFromContext(r)
	appID := r.URL.Query().Get("app_id")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")
	pageStr := r.URL.Query().Get("page")
	limit := 15
	if limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 {
			limit = n
		}
	}
	offset := 0
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 1 {
			offset = (p - 1) * limit
		}
	} else if offsetStr != "" {
		if n, err := strconv.Atoi(offsetStr); err == nil && n >= 0 {
			offset = n
		}
	}

	if user.IsAdmin {
		runs, err := s.store.ListRuns(appID, limit, offset)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		total, err := s.store.CountRuns(appID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"runs": runs, "total": total})
		return
	}

	allowed, allowedList, err := s.allowedAppIDsForUser(user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if appID != "" {
		if _, ok := allowed[appID]; !ok {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "user has no access to this app"})
			return
		}
		runs, err := s.store.ListRuns(appID, limit, offset)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		total, err := s.store.CountRuns(appID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"runs": runs, "total": total})
		return
	}

	runs, err := s.store.ListRunsByAppIDs(allowedList, limit, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	total, err := s.store.CountRunsByAppIDs(allowedList)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"runs": runs, "total": total})
}

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
	user := authUserFromContext(r)
	if !user.IsAdmin {
		ok, err := s.userCanAccessApp(user.ID, run.AppID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if !ok {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "user has no access to this run"})
			return
		}
	}
	writeJSON(w, http.StatusOK, run)
}

func (s *Server) readSessionUser(r *http.Request) (authUser, string, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return authUser{}, "", false
	}
	token := cookie.Value
	now := time.Now()
	s.sessionsMu.RLock()
	session, ok := s.sessions[token]
	s.sessionsMu.RUnlock()
	if !ok {
		return authUser{}, "", false
	}
	if !session.ExpiresAt.After(now) {
		s.invalidateSession(token)
		return authUser{}, "", false
	}
	return session.User, token, true
}

func (s *Server) authenticateSession(w http.ResponseWriter, r *http.Request, rotate bool) (authUser, string, bool) {
	u, token, ok := s.readSessionUser(r)
	if !ok {
		return authUser{}, "", false
	}
	if rotate {
		s.sessionsMu.RLock()
		session := s.sessions[token]
		s.sessionsMu.RUnlock()
		if time.Until(session.ExpiresAt) <= sessionRotateThreshold {
			s.invalidateSession(token)
			if err := s.createSession(w, u); err != nil {
				return authUser{}, "", false
			}
		}
	}
	return u, token, true
}

func (s *Server) allowedAppIDsForUser(userID int64) (map[string]struct{}, []string, error) {
	groupIDs, err := s.store.UserGroupIDs(userID)
	if err != nil {
		return nil, nil, err
	}
	appIDs, err := s.store.AppIDsByUserGroupIDs(groupIDs)
	if err != nil {
		return nil, nil, err
	}
	allowed := make(map[string]struct{}, len(appIDs))
	for _, appID := range appIDs {
		allowed[appID] = struct{}{}
	}
	return allowed, appIDs, nil
}

func (s *Server) userCanAccessApp(userID int64, appID string) (bool, error) {
	allowed, _, err := s.allowedAppIDsForUser(userID)
	if err != nil {
		return false, err
	}
	_, ok := allowed[appID]
	return ok, nil
}

func (s *Server) appExists(appID string) bool {
	s.appsMu.RLock()
	defer s.appsMu.RUnlock()
	for _, a := range s.apps {
		if a.ID == appID {
			return true
		}
	}
	return false
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func storeErrNoRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

func (s *Server) createSession(w http.ResponseWriter, user authUser) error {
	token, err := randomToken()
	if err != nil {
		return err
	}
	exp := time.Now().Add(time.Duration(sessionTTLSeconds) * time.Second)
	s.sessionsMu.Lock()
	s.sessions[token] = sessionData{User: user, ExpiresAt: exp}
	s.sessionsMu.Unlock()
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   sessionTTLSeconds,
	})
	return nil
}

func (s *Server) invalidateSession(token string) {
	if strings.TrimSpace(token) == "" {
		return
	}
	s.sessionsMu.Lock()
	delete(s.sessions, token)
	s.sessionsMu.Unlock()
}

func (s *Server) invalidateUserSessions(userID int64) {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()
	for token, session := range s.sessions {
		if session.User.ID == userID {
			delete(s.sessions, token)
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"piaflow/internal/auth"
	"piaflow/internal/config"
	"piaflow/internal/pipeline"
	"piaflow/internal/store"
)

func TestServer_AuthRequiredForAPI(t *testing.T) {
	h, _, _, _ := setupTestServer(t, []config.App{
		{ID: "app-a", Name: "App A", Repo: "https://example.com/a.git", Branch: "main", TestCmd: "echo test", BuildCmd: "echo build"},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/apps", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestServer_LoginAndProfile(t *testing.T) {
	h, _, _, _ := setupTestServer(t, []config.App{
		{ID: "app-a", Name: "App A", Repo: "https://example.com/a.git", Branch: "main", TestCmd: "echo test", BuildCmd: "echo build"},
	})

	cookie := loginAndCookie(t, h, "admin", "admin")

	reqMe := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	reqMe.AddCookie(cookie)
	recMe := httptest.NewRecorder()
	h.ServeHTTP(recMe, reqMe)
	if recMe.Code != http.StatusOK {
		t.Fatalf("expected 200 on /api/auth/me, got %d", recMe.Code)
	}

	reqProfile := httptest.NewRequest(http.MethodGet, "/api/auth/profile", nil)
	reqProfile.AddCookie(cookie)
	recProfile := httptest.NewRecorder()
	h.ServeHTTP(recProfile, reqProfile)
	if recProfile.Code != http.StatusOK {
		t.Fatalf("expected 200 on /api/auth/profile, got %d", recProfile.Code)
	}
}

func TestServer_NonAdminGroupAppAccessAndEdit(t *testing.T) {
	apps := []config.App{
		{ID: "app-a", Name: "App A", Repo: "https://example.com/a.git", Branch: "main", TestCmd: "echo test", BuildCmd: "echo build"},
		{ID: "app-b", Name: "App B", Repo: "https://example.com/b.git", Branch: "main", TestCmd: "echo test", BuildCmd: "echo build"},
	}
	h, st, _, _ := setupTestServer(t, apps)

	devID, err := st.CreateGroup("dev")
	if err != nil {
		t.Fatal(err)
	}
	hash, err := auth.HashPassword("alice123")
	if err != nil {
		t.Fatal(err)
	}
	aliceID, err := st.CreateUser("alice", hash, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetUserGroups(aliceID, []int64{devID}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetAppGroups("app-a", []int64{devID}); err != nil {
		t.Fatal(err)
	}

	aliceCookie := loginAndCookie(t, h, "alice", "alice123")

	reqList := httptest.NewRequest(http.MethodGet, "/api/apps", nil)
	reqList.AddCookie(aliceCookie)
	recList := httptest.NewRecorder()
	h.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 listing apps, got %d", recList.Code)
	}
	var listed []map[string]interface{}
	if err := json.NewDecoder(recList.Body).Decode(&listed); err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0]["id"] != "app-a" {
		t.Fatalf("expected only app-a for alice, got %+v", listed)
	}

	updateBody := map[string]interface{}{
		"name": "App A Updated", "repo": "https://example.com/a.git", "branch": "main",
		"test_cmd": "echo test", "build_cmd": "echo build", "deploy_cmd": "",
	}
	bodyBytes, _ := json.Marshal(updateBody)
	reqUpdateAllowed := httptest.NewRequest(http.MethodPut, "/api/apps/app-a", bytes.NewReader(bodyBytes))
	reqUpdateAllowed.Header.Set("Content-Type", "application/json")
	reqUpdateAllowed.AddCookie(aliceCookie)
	recUpdateAllowed := httptest.NewRecorder()
	h.ServeHTTP(recUpdateAllowed, reqUpdateAllowed)
	if recUpdateAllowed.Code != http.StatusOK {
		t.Fatalf("expected 200 updating allowed app, got %d", recUpdateAllowed.Code)
	}

	reqUpdateDenied := httptest.NewRequest(http.MethodPut, "/api/apps/app-b", bytes.NewReader(bodyBytes))
	reqUpdateDenied.Header.Set("Content-Type", "application/json")
	reqUpdateDenied.AddCookie(aliceCookie)
	recUpdateDenied := httptest.NewRecorder()
	h.ServeHTTP(recUpdateDenied, reqUpdateDenied)
	if recUpdateDenied.Code != http.StatusForbidden {
		t.Fatalf("expected 403 updating denied app, got %d", recUpdateDenied.Code)
	}
}

func TestServer_ChangeOwnPassword(t *testing.T) {
	h, st, _, _ := setupTestServer(t, []config.App{
		{ID: "app-a", Name: "App A", Repo: "https://example.com/a.git", Branch: "main", TestCmd: "echo test", BuildCmd: "echo build"},
	})
	hash, err := auth.HashPassword("bob-old")
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.CreateUser("bob", hash, false)
	if err != nil {
		t.Fatal(err)
	}

	bobCookie := loginAndCookie(t, h, "bob", "bob-old")
	changeBody := map[string]string{"current_password": "bob-old", "new_password": "bob-new"}
	changeBytes, _ := json.Marshal(changeBody)
	reqChange := httptest.NewRequest(http.MethodPut, "/api/auth/password", bytes.NewReader(changeBytes))
	reqChange.Header.Set("Content-Type", "application/json")
	reqChange.AddCookie(bobCookie)
	recChange := httptest.NewRecorder()
	h.ServeHTTP(recChange, reqChange)
	if recChange.Code != http.StatusOK {
		t.Fatalf("expected 200 on password change, got %d", recChange.Code)
	}

	loginFailBody, _ := json.Marshal(map[string]string{"username": "bob", "password": "bob-old"})
	reqLoginFail := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginFailBody))
	reqLoginFail.Header.Set("Content-Type", "application/json")
	recLoginFail := httptest.NewRecorder()
	h.ServeHTTP(recLoginFail, reqLoginFail)
	if recLoginFail.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 login with old password, got %d", recLoginFail.Code)
	}

	_ = loginAndCookie(t, h, "bob", "bob-new")
}

func TestServer_DeleteAppAlsoDeletesRuns_AndAdminDeleteBlocked(t *testing.T) {
	apps := []config.App{
		{ID: "app-a", Name: "App A", Repo: "https://example.com/a.git", Branch: "main", TestCmd: "echo test", BuildCmd: "echo build"},
		{ID: "app-b", Name: "App B", Repo: "https://example.com/b.git", Branch: "main", TestCmd: "echo test", BuildCmd: "echo build"},
	}
	h, st, _, _ := setupTestServer(t, apps)
	adminCookie := loginAndCookie(t, h, "admin", "admin")

	if _, err := st.CreateRun("app-a", "", "admin"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateRun("app-a", "", "admin"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateRun("app-b", "", "admin"); err != nil {
		t.Fatal(err)
	}

	reqDeleteApp := httptest.NewRequest(http.MethodDelete, "/api/apps/app-a", nil)
	reqDeleteApp.AddCookie(adminCookie)
	recDeleteApp := httptest.NewRecorder()
	h.ServeHTTP(recDeleteApp, reqDeleteApp)
	if recDeleteApp.Code != http.StatusNoContent {
		t.Fatalf("expected 204 deleting app-a, got %d", recDeleteApp.Code)
	}
	countA, err := st.CountRuns("app-a")
	if err != nil {
		t.Fatal(err)
	}
	if countA != 0 {
		t.Fatalf("expected 0 runs for app-a, got %d", countA)
	}
	countB, err := st.CountRuns("app-b")
	if err != nil {
		t.Fatal(err)
	}
	if countB != 1 {
		t.Fatalf("expected 1 run for app-b, got %d", countB)
	}

	otherAdminHash, err := auth.HashPassword("admin2")
	if err != nil {
		t.Fatal(err)
	}
	otherAdminID, err := st.CreateUser("admin2", otherAdminHash, true)
	if err != nil {
		t.Fatal(err)
	}
	reqDeleteAdmin := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/users/%d", otherAdminID), nil)
	reqDeleteAdmin.AddCookie(adminCookie)
	recDeleteAdmin := httptest.NewRecorder()
	h.ServeHTTP(recDeleteAdmin, reqDeleteAdmin)
	if recDeleteAdmin.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 deleting admin user, got %d", recDeleteAdmin.Code)
	}
}

func setupTestServer(t *testing.T, apps []config.App) (http.Handler, *store.Store, string, string) {
	t.Helper()
	baseDir := t.TempDir()
	dbPath := filepath.Join(baseDir, "test.db")
	appsPath := filepath.Join(baseDir, "apps.yaml")
	staticDir := filepath.Join(baseDir, "web")

	if err := config.SaveApps(appsPath, apps); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(staticDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staticDir, "index.html"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	st, err := store.New("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	adminHash, err := auth.HashPassword("admin")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.EnsureAdminUser("admin", adminHash); err != nil {
		t.Fatal(err)
	}

	runner := pipeline.NewRunner(filepath.Join(baseDir, "work"))
	srv := New(apps, st, runner, appsPath, staticDir)
	return srv.Handler(), st, appsPath, staticDir
}

func loginAndCookie(t *testing.T, h http.Handler, username, password string) *http.Cookie {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"username": username, "password": password})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login failed for %s, status=%d body=%s", username, rec.Code, rec.Body.String())
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookieName {
			return c
		}
	}
	t.Fatalf("no %s cookie in login response", sessionCookieName)
	return nil
}

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadApps(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "apps.yaml")
	content := `
apps:
  - id: test-app
    name: Test App
    repo: https://github.com/org/repo.git
    branch: main
    build_cmd: go build .
    test_cmd: go test ./...
    deploy_cmd: ""
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	apps, err := LoadApps(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) != 1 {
		t.Fatalf("expected 1 app, got %d", len(apps))
	}
	if apps[0].ID != "test-app" || apps[0].Name != "Test App" {
		t.Errorf("unexpected app: %+v", apps[0])
	}
}

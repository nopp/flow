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

func TestEffectiveStepsFromLegacyFields(t *testing.T) {
	app := App{
		ID:            "legacy",
		TestCmd:       "go test ./...",
		BuildCmd:      "go build ./...",
		DeployCmd:     "./deploy.sh",
		TestSleepSec:  2,
		BuildSleepSec: 3,
	}
	steps := app.EffectiveSteps()
	if len(steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(steps))
	}
	if steps[0].Name != "test" || steps[0].Cmd != "go test ./..." || steps[0].SleepSec != 2 {
		t.Fatalf("unexpected test step: %+v", steps[0])
	}
	if steps[1].Name != "build" || steps[1].Cmd != "go build ./..." || steps[1].SleepSec != 3 {
		t.Fatalf("unexpected build step: %+v", steps[1])
	}
	if steps[2].Name != "deploy" || steps[2].Cmd != "./deploy.sh" {
		t.Fatalf("unexpected deploy step: %+v", steps[2])
	}
}

func TestEffectiveStepsFromDynamicList(t *testing.T) {
	app := App{
		ID: "dynamic",
		Steps: []Step{
			{Name: "lint", Cmd: "go vet ./...", SleepSec: 1},
			{Name: "", Cmd: "go test ./..."},
			{Name: "empty", Cmd: "   "},
		},
	}
	steps := app.EffectiveSteps()
	if len(steps) != 2 {
		t.Fatalf("expected 2 non-empty steps, got %d", len(steps))
	}
	if steps[0].Name != "lint" || steps[0].Cmd != "go vet ./..." || steps[0].SleepSec != 1 {
		t.Fatalf("unexpected first step: %+v", steps[0])
	}
	if steps[1].Name != "step-2" || steps[1].Cmd != "go test ./..." {
		t.Fatalf("unexpected second step: %+v", steps[1])
	}
}

func TestEffectiveStepsSupportsFileAndScript(t *testing.T) {
	app := App{
		ID: "dynamic-kinds",
		Steps: []Step{
			{Name: "deploy-file", File: "scripts/deploy.sh"},
			{Name: "inline", Script: "echo inline"},
			{Name: "invalid", Cmd: "echo one", File: "script.sh"},
		},
	}
	steps := app.EffectiveSteps()
	if len(steps) != 2 {
		t.Fatalf("expected 2 valid steps, got %d", len(steps))
	}
	if steps[0].Kind() != "file" || steps[0].File != "scripts/deploy.sh" {
		t.Fatalf("unexpected first step: %+v", steps[0])
	}
	if steps[1].Kind() != "script" || steps[1].Script != "echo inline" {
		t.Fatalf("unexpected second step: %+v", steps[1])
	}
}

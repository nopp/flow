package server

import (
	"strings"
	"testing"

	"noppflow/internal/config"
)

func TestBuildK8sJobScript_AddsKubectlEnsure(t *testing.T) {
	app := config.App{
		ID:                 "app-1",
		Repo:               "https://example.com/repo.git",
		Branch:             "main",
		DeployMode:         "kubectl",
		K8sNamespace:       "apps",
		DeployManifestPath: "k8s/",
		Steps: []config.Step{
			{Name: "deploy_apply", K8sDeploy: true},
		},
	}

	script := buildK8sJobScript(app, nil, false)
	if !strings.Contains(script, "workspace repo missing; restoring from origin") {
		t.Fatalf("expected repo restore guard in script, got:\n%s", script)
	}
	if !strings.Contains(script, "if ! command -v kubectl >/dev/null 2>&1; then") {
		t.Fatalf("expected kubectl ensure block in script, got:\n%s", script)
	}
	if !strings.Contains(script, "s/xxnamexx/app-1/g") {
		t.Fatalf("expected placeholder substitution in kubectl deploy script, got:\n%s", script)
	}
	if strings.Index(script, "=== Step: deploy_apply ===") > strings.Index(script, "if ! command -v kubectl >/dev/null 2>&1; then") {
		t.Fatalf("expected kubectl ensure block to run during deploy step, got:\n%s", script)
	}
	if !strings.Contains(script, "kubectl -n 'apps' apply -f 'k8s/'") {
		t.Fatalf("expected kubectl apply command in script, got:\n%s", script)
	}
}

func TestBuildK8sJobScript_AddsHelmEnsure(t *testing.T) {
	app := config.App{
		ID:           "app-1",
		Repo:         "https://example.com/repo.git",
		Branch:       "main",
		DeployMode:   "helm",
		K8sNamespace: "apps",
		HelmChart:    "./charts/app",
		Steps: []config.Step{
			{Name: "deploy_apply", K8sDeploy: true},
		},
	}

	script := buildK8sJobScript(app, nil, false)
	if !strings.Contains(script, "workspace repo missing; restoring from origin") {
		t.Fatalf("expected repo restore guard in script, got:\n%s", script)
	}
	if !strings.Contains(script, "if ! command -v helm >/dev/null 2>&1; then") {
		t.Fatalf("expected helm ensure block in script, got:\n%s", script)
	}
	if strings.Index(script, "=== Step: deploy_apply ===") > strings.Index(script, "if ! command -v helm >/dev/null 2>&1; then") {
		t.Fatalf("expected helm ensure block to run during deploy step, got:\n%s", script)
	}
	if !strings.Contains(script, "helm upgrade --install 'app-1' './charts/app' -n 'apps'") {
		t.Fatalf("expected helm command in script, got:\n%s", script)
	}
}

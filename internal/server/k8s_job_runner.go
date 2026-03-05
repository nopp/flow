package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"noppflow/internal/config"
	"noppflow/internal/pipeline"
)

const k8sRunTimeout = 30 * time.Minute

func appUsesK8sJob(app config.App) bool {
	for _, step := range app.EffectiveSteps() {
		if step.Kind() == "k8s_deploy" {
			return true
		}
	}
	return false
}

func (s *Server) runAppAsK8sJob(runID int64, app config.App, privateKey string, stepEnv map[string]string, onLogUpdate func(log string)) pipeline.Result {
	namespace := strings.TrimSpace(app.K8sNamespace)
	if namespace == "" {
		return pipeline.Result{Success: false, Log: "k8s namespace is required"}
	}
	serviceAccount := strings.TrimSpace(app.K8sServiceAccount)
	if serviceAccount == "" {
		return pipeline.Result{Success: false, Log: "k8s service account is required"}
	}
	runnerImage := strings.TrimSpace(app.K8sRunnerImage)
	if runnerImage == "" {
		return pipeline.Result{Success: false, Log: "k8s runner image is required"}
	}
	if err := ensureKubectlPresent(); err != nil {
		return pipeline.Result{Success: false, Log: err.Error()}
	}

	jobName := fmt.Sprintf("noppflow-run-%d", runID)
	secretName := jobName + "-ssh"
	useSSHKey := strings.TrimSpace(privateKey) != ""
	script := buildK8sJobScript(app, stepEnv, useSSHKey)
	if strings.TrimSpace(script) == "" {
		return pipeline.Result{Success: false, Log: "empty k8s job script"}
	}

	if useSSHKey {
		secretYAML := buildK8sRunSecretYAML(namespace, secretName, privateKey)
		if err := kubectlApplyYAML(secretYAML); err != nil {
			return pipeline.Result{Success: false, Log: fmt.Sprintf("failed to create ssh secret: %v", err)}
		}
		defer func() { _ = kubectlDeleteResource(namespace, "secret", secretName) }()
	}

	jobYAML := buildK8sRunJobYAML(namespace, jobName, serviceAccount, runnerImage, secretName, script, useSSHKey)
	if err := kubectlApplyYAML(jobYAML); err != nil {
		return pipeline.Result{Success: false, Log: fmt.Sprintf("failed to create job: %v", err)}
	}

	ctx, cancel := context.WithTimeout(context.Background(), k8sRunTimeout)
	defer cancel()

	lastLog := ""
	for {
		log, err := kubectlJobLogs(namespace, jobName)
		if err == nil {
			if log != lastLog {
				lastLog = log
				if onLogUpdate != nil {
					onLogUpdate(lastLog)
				}
			}
		}

		done, success, err := kubectlJobDone(namespace, jobName)
		if err == nil && done {
			if success {
				return pipeline.Result{Success: true, Log: lastLog}
			}
			return pipeline.Result{Success: false, Log: lastLog}
		}

		select {
		case <-ctx.Done():
			if lastLog == "" {
				lastLog = "k8s job timed out"
			} else {
				lastLog += "\n\nk8s job timed out"
			}
			return pipeline.Result{Success: false, Log: lastLog}
		case <-time.After(2 * time.Second):
		}
	}
}

func ensureKubectlPresent() error {
	if _, err := exec.LookPath("kubectl"); err != nil {
		return fmt.Errorf("kubectl is required in noppflow api container but was not found in PATH=%s", os.Getenv("PATH"))
	}
	return nil
}

func kubectlApplyYAML(yamlBody string) error {
	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(yamlBody)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return err
		}
		return fmt.Errorf("%w: %s", err, msg)
	}
	return nil
}

func kubectlDeleteResource(namespace, kind, name string) error {
	cmd := exec.Command("kubectl", "-n", namespace, "delete", kind, name, "--ignore-not-found=true")
	return cmd.Run()
}

func kubectlOutput(args ...string) (string, error) {
	cmd := exec.Command("kubectl", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("kubectl %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func kubectlJobDone(namespace, jobName string) (done bool, success bool, err error) {
	succeeded, err := kubectlOutput("-n", namespace, "get", "job", jobName, "-o", "jsonpath={.status.succeeded}")
	if err != nil {
		return false, false, err
	}
	if succeeded != "" && succeeded != "0" {
		return true, true, nil
	}
	failed, err := kubectlOutput("-n", namespace, "get", "job", jobName, "-o", "jsonpath={.status.failed}")
	if err != nil {
		return false, false, err
	}
	if failed != "" && failed != "0" {
		return true, false, nil
	}
	return false, false, nil
}

func kubectlJobLogs(namespace, jobName string) (string, error) {
	podName, err := kubectlOutput("-n", namespace, "get", "pods", "-l", "job-name="+jobName, "-o", "jsonpath={.items[0].metadata.name}")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(podName) == "" {
		return "", fmt.Errorf("pod not ready")
	}
	return kubectlOutput("-n", namespace, "logs", podName, "--tail=-1")
}

func buildK8sRunSecretYAML(namespace, secretName, privateKey string) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(privateKey))
	return fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: %s
  namespace: %s
type: Opaque
data:
  id_key: %s
`, secretName, namespace, encoded)
}

func buildK8sRunJobYAML(namespace, jobName, serviceAccount, image, secretName, script string, useSSHKey bool) string {
	volumeMounts := ""
	volumes := ""
	if useSSHKey {
		volumeMounts = `          volumeMounts:
            - name: ssh-key
              mountPath: /var/run/noppflow-ssh
              readOnly: true
`
		volumes = fmt.Sprintf(`      volumes:
        - name: ssh-key
          secret:
            secretName: %s
`, secretName)
	}
	return fmt.Sprintf(`apiVersion: batch/v1
kind: Job
metadata:
  name: %s
  namespace: %s
spec:
  backoffLimit: 0
  ttlSecondsAfterFinished: 3600
  template:
    metadata:
      labels:
        app: noppflow-runner
    spec:
      restartPolicy: Never
      serviceAccountName: %s
      containers:
        - name: runner
          image: %s
          imagePullPolicy: IfNotPresent
          command:
            - /bin/sh
            - -c
            - |
%s
%s%s`, jobName, namespace, serviceAccount, image, indentYAMLBlock(script, 14), volumeMounts, volumes)
}

func buildK8sJobScript(app config.App, stepEnv map[string]string, useSSHKey bool) string {
	steps := app.EffectiveSteps()
	lines := []string{
		"set -eu",
		"mkdir -p /workspace",
		"cd /workspace",
		"export GIT_TERMINAL_PROMPT=0",
	}
	if useSSHKey {
		lines = append(lines, fmt.Sprintf("export GIT_SSH_COMMAND=%s", shellQuote("ssh -i /var/run/noppflow-ssh/id_key -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new")))
	}
	lines = append(lines,
		fmt.Sprintf("git clone --branch %s --single-branch %s repo", shellQuote(app.Branch), shellQuote(app.Repo)),
		"cd repo",
	)
	for k, v := range stepEnv {
		name := strings.TrimSpace(k)
		if name == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("export %s=%s", name, shellQuote(v)))
	}
	// Ensure core system paths remain available even if a global env var overrides PATH.
	lines = append(lines, "export PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:${PATH:-}")
	deployMode := strings.TrimSpace(strings.ToLower(app.DeployMode))
	for _, step := range steps {
		lines = append(lines, fmt.Sprintf("echo %s", shellQuote("=== Step: "+step.Name+" ===")))
		lines = append(lines, fmt.Sprintf("if [ ! -d /workspace/repo/.git ]; then echo %s; cd /; mkdir -p /workspace; rm -rf /workspace/repo; git clone --branch %s --single-branch %s /workspace/repo; fi", shellQuote("workspace repo missing; restoring from origin"), shellQuote(app.Branch), shellQuote(app.Repo)))
		lines = append(lines, "if [ -d /workspace/repo ]; then cd /workspace/repo; elif [ -d /workspace ]; then cd /workspace; else cd /; fi")
		switch step.Kind() {
		case "cmd":
			lines = append(lines, fmt.Sprintf("sh -c %s", shellQuote(step.Cmd)))
		case "file":
			lines = append(lines, fmt.Sprintf("sh %s", shellQuote(step.File)))
		case "script":
			lines = append(lines, fmt.Sprintf("printf %%s %s | sh", shellQuote(step.Script)))
		case "k8s_deploy":
			if deployMode == "kubectl" {
				lines = appendEnsureBinary(lines, "kubectl")
				manifestPath := shellQuote(app.DeployManifestPath)
				deployName := strings.TrimSpace(app.Name)
				if deployName == "" {
					deployName = strings.TrimSpace(app.ID)
				}
				sedExpr := shellQuote("s/xxnamexx/" + sedEscape(deployName) + "/g")
				lines = append(lines, fmt.Sprintf("if [ -d %s ]; then find %s -type f \\( -name '*.yaml' -o -name '*.yml' \\) -exec sed -i %s {} +; elif [ -f %s ]; then sed -i %s %s; fi", manifestPath, manifestPath, sedExpr, manifestPath, sedExpr, manifestPath))
				lines = append(lines, fmt.Sprintf("kubectl -n %s apply -f %s", shellQuote(app.K8sNamespace), shellQuote(app.DeployManifestPath)))
			} else if deployMode == "helm" {
				lines = appendEnsureBinary(lines, "helm")
				helmCmd := fmt.Sprintf("helm upgrade --install %s %s -n %s", shellQuote(app.ID), shellQuote(app.HelmChart), shellQuote(app.K8sNamespace))
				if strings.TrimSpace(app.HelmValuesPath) != "" {
					helmCmd += fmt.Sprintf(" -f %s", shellQuote(app.HelmValuesPath))
				}
				lines = append(lines, helmCmd)
			}
		}
		lines = append(lines, fmt.Sprintf("echo %s", shellQuote(step.Name+" step OK")))
		if step.SleepSec > 0 {
			lines = append(lines, fmt.Sprintf("sleep %d", step.SleepSec))
		}
	}
	lines = append(lines, "echo 'pipeline completed successfully'")
	return strings.Join(lines, "\n")
}

func appendEnsureBinary(lines []string, bin string) []string {
	lines = append(lines, fmt.Sprintf("if ! command -v %s >/dev/null 2>&1; then", bin))
	lines = append(lines, fmt.Sprintf("  echo %s", shellQuote(bin+" not found in runner image; attempting installation")))
	lines = append(lines, fmt.Sprintf("  if command -v apk >/dev/null 2>&1; then apk add --no-cache %s || true; \\", bin))
	lines = append(lines, fmt.Sprintf("  elif command -v apt-get >/dev/null 2>&1; then apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends %s || true; \\", bin))
	lines = append(lines, fmt.Sprintf("  elif command -v microdnf >/dev/null 2>&1; then microdnf install -y %s || true; \\", bin))
	lines = append(lines, fmt.Sprintf("  elif command -v dnf >/dev/null 2>&1; then dnf install -y %s || true; \\", bin))
	lines = append(lines, fmt.Sprintf("  elif command -v yum >/dev/null 2>&1; then yum install -y %s || true; \\", bin))
	lines = append(lines, "  else true; fi")
	lines = append(lines, fmt.Sprintf("  if ! command -v %s >/dev/null 2>&1; then", bin))
	lines = append(lines, fmt.Sprintf("    echo %s", shellQuote(bin+" still unavailable after package manager attempt; trying direct download")))
	lines = append(lines, "    arch=\"$(uname -m)\"")
	lines = append(lines, "    case \"$arch\" in x86_64|amd64) arch=amd64 ;; aarch64|arm64) arch=arm64 ;; *) arch=\"\" ;; esac")
	lines = append(lines, "    if [ -n \"$arch\" ] && command -v curl >/dev/null 2>&1; then")
	lines = append(lines, "      if [ \""+bin+"\" = \"kubectl\" ]; then")
	lines = append(lines, "        curl -fsSL \"https://dl.k8s.io/release/v1.30.8/bin/linux/${arch}/kubectl\" -o /tmp/kubectl || true")
	lines = append(lines, "        if [ -s /tmp/kubectl ]; then chmod +x /tmp/kubectl && cp /tmp/kubectl /usr/local/bin/kubectl || true; fi")
	lines = append(lines, "      elif [ \""+bin+"\" = \"helm\" ]; then")
	lines = append(lines, "        curl -fsSL \"https://get.helm.sh/helm-v3.16.4-linux-${arch}.tar.gz\" -o /tmp/helm.tgz || true")
	lines = append(lines, "        if [ -s /tmp/helm.tgz ]; then tar -xzf /tmp/helm.tgz -C /tmp && cp \"/tmp/linux-${arch}/helm\" /usr/local/bin/helm || true; fi")
	lines = append(lines, "      fi")
	lines = append(lines, "    fi")
	lines = append(lines, "  fi")
	lines = append(lines, "fi")
	lines = append(lines, fmt.Sprintf("command -v %s >/dev/null 2>&1 || { echo %s; exit 127; }", bin, shellQuote(bin+" unavailable after installation attempt")))
	return lines
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

func sedEscape(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "&", "\\&")
	s = strings.ReplaceAll(s, "/", "\\/")
	return s
}

func indentYAMLBlock(s string, spaces int) string {
	prefix := strings.Repeat(" ", spaces)
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = prefix + lines[i]
	}
	return strings.Join(lines, "\n")
}

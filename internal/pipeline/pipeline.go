// Package pipeline runs the CI/CD steps for an app: clone (or pull), test, build, and optionally deploy.
// Each run executes in a subdirectory of the runner's work dir (work/<app_id>/).
// Commands are parsed with splitCommand to support quoted arguments.
package pipeline

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"piaflow/internal/config"
)

// Runner holds the base directory where app repositories are cloned (e.g. work/).
// Each app gets workDir/<app.ID>/ as its working directory.
type Runner struct {
	workDir string
}

// NewRunner creates a pipeline runner. workDir is where repos are cloned (e.g. ./work).
func NewRunner(workDir string) *Runner {
	return &Runner{workDir: workDir}
}

// Result holds the outcome of a pipeline run.
type Result struct {
	Success bool
	Log     string
}

// RunOptions configures runtime behavior for a pipeline run.
type RunOptions struct {
	GitSSHCommand string
}

// Run executes clone, test, build, and optionally deploy for the given app.
// If onLogUpdate is non-nil, it is called with the current log after each step so the UI can stream it.
func (r *Runner) Run(app config.App, opts RunOptions, onLogUpdate func(log string)) Result {
	var log bytes.Buffer
	appendLog := func(format string, args ...interface{}) {
		log.WriteString(fmt.Sprintf(format+"\n", args...))
		if onLogUpdate != nil {
			onLogUpdate(log.String())
		}
	}
	gitEnv := []string(nil)
	if strings.TrimSpace(opts.GitSSHCommand) != "" {
		gitEnv = append(os.Environ(), "GIT_SSH_COMMAND="+opts.GitSSHCommand)
	}

	appWorkDir := filepath.Join(r.workDir, app.ID)
	if err := os.MkdirAll(r.workDir, 0755); err != nil {
		appendLog("mkdir work dir: %v", err)
		return Result{Success: false, Log: log.String()}
	}

	// Clone or pull
	if _, err := os.Stat(filepath.Join(appWorkDir, ".git")); err != nil {
		if err := os.MkdirAll(appWorkDir, 0755); err != nil {
			appendLog("mkdir app dir: %v", err)
			return Result{Success: false, Log: log.String()}
		}
		if err := r.runCmd(gitEnv, appWorkDir, "git", "clone", "--branch", app.Branch, "--single-branch", app.Repo, "."); err != nil {
			appendLog("git clone: %v", err)
			return Result{Success: false, Log: log.String()}
		}
	} else {
		if err := r.runCmd(gitEnv, appWorkDir, "git", "pull", "origin", app.Branch); err != nil {
			appendLog("git pull: %v", err)
			return Result{Success: false, Log: log.String()}
		}
	}

	commit, _ := r.output(gitEnv, appWorkDir, "git", "rev-parse", "HEAD")
	appendLog("commit: %s", strings.TrimSpace(commit))

	steps := app.EffectiveSteps()
	for _, step := range steps {
		appendLog("=== Step: %s ===", step.Name)
		if err := r.runCmdWithLog(appWorkDir, step.Cmd, &log); err != nil {
			if onLogUpdate != nil {
				onLogUpdate(log.String())
			}
			appendLog("%s step failed: %v", step.Name, err)
			return Result{Success: false, Log: log.String()}
		}
		appendLog("%s step OK", step.Name)
		if step.SleepSec > 0 {
			appendLog("Sleeping %ds after %s...", step.SleepSec, step.Name)
			time.Sleep(time.Duration(step.SleepSec) * time.Second)
			if onLogUpdate != nil {
				onLogUpdate(log.String())
			}
		}
	}

	appendLog("pipeline completed successfully")
	return Result{Success: true, Log: log.String()}
}

// runCmd runs a command in dir with stdout/stderr attached to the process (for git clone/pull).
func (r *Runner) runCmd(env []string, dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = env
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// runCmdWithLog runs a shell command (parsed by splitCommand) in dir and writes stdout/stderr to log.
func (r *Runner) runCmdWithLog(dir, command string, log *bytes.Buffer) error {
	parts := splitCommand(command)
	if len(parts) == 0 {
		return nil
	}
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = dir
	cmd.Stdout = log
	cmd.Stderr = log
	return cmd.Run()
}

// output runs a command in dir and returns its combined stdout.
func (r *Runner) output(env []string, dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = env
	}
	out, err := cmd.Output()
	return string(out), err
}

// splitCommand splits a command string into parts, respecting single and double quotes.
// Used to parse TestCmd, BuildCmd, and DeployCmd from the app config.
func splitCommand(s string) []string {
	var parts []string
	var buf strings.Builder
	quote := false
	for _, r := range s {
		switch r {
		case ' ':
			if !quote {
				if buf.Len() > 0 {
					parts = append(parts, buf.String())
					buf.Reset()
				}
			} else {
				buf.WriteRune(r)
			}
		case '"', '\'':
			quote = !quote
		default:
			buf.WriteRune(r)
		}
	}
	if buf.Len() > 0 {
		parts = append(parts, buf.String())
	}
	return parts
}

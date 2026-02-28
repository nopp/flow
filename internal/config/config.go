// Package config loads and saves application definitions from and to YAML.
// The main file is typically config/apps.yaml; its path is given at startup.
// Apps are also persisted when created, updated, or deleted via the API.
package config

import (
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Step defines one pipeline step.
type Step struct {
	Name     string `yaml:"name" json:"name"`
	Cmd      string `yaml:"cmd" json:"cmd"`
	File     string `yaml:"file,omitempty" json:"file,omitempty"`
	Script   string `yaml:"script,omitempty" json:"script,omitempty"`
	SleepSec int    `yaml:"sleep_sec" json:"sleep_sec"`
}

// Kind returns which execution mode this step uses.
// It returns "cmd", "file", "script", or "" when none/invalid.
func (s Step) Kind() string {
	cmd := strings.TrimSpace(s.Cmd)
	file := strings.TrimSpace(s.File)
	script := strings.TrimSpace(s.Script)
	count := 0
	kind := ""
	if cmd != "" {
		count++
		kind = "cmd"
	}
	if file != "" {
		count++
		kind = "file"
	}
	if script != "" {
		count++
		kind = "script"
	}
	if count != 1 {
		return ""
	}
	return kind
}

// CommandValue returns the value for the configured Kind.
func (s Step) CommandValue() string {
	switch s.Kind() {
	case "cmd":
		return strings.TrimSpace(s.Cmd)
	case "file":
		return strings.TrimSpace(s.File)
	case "script":
		return strings.TrimSpace(s.Script)
	default:
		return ""
	}
}

// App defines a single application in the CI/CD system.
// ID is unique and used in URLs and as the clone directory name under work/.
// Repo is the git clone URL; Branch defaults to "main" if empty.
// TestCmd and BuildCmd are required; DeployCmd is optional.
// TestSleepSec, BuildSleepSec, DeploySleepSec are optional: when > 0, the pipeline sleeps that many seconds after the corresponding step.
type App struct {
	ID             string `yaml:"id" json:"id"`
	Name           string `yaml:"name" json:"name"`
	Repo           string `yaml:"repo" json:"repo"`
	Branch         string `yaml:"branch" json:"branch"`
	SSHKeyName     string `yaml:"ssh_key_name,omitempty" json:"ssh_key_name,omitempty"`
	Steps          []Step `yaml:"steps,omitempty" json:"steps,omitempty"`
	BuildCmd       string `yaml:"build_cmd,omitempty" json:"build_cmd,omitempty"`
	TestCmd        string `yaml:"test_cmd,omitempty" json:"test_cmd,omitempty"`
	DeployCmd      string `yaml:"deploy_cmd,omitempty" json:"deploy_cmd,omitempty"`
	TestSleepSec   int    `yaml:"test_sleep_sec,omitempty" json:"test_sleep_sec,omitempty"`
	BuildSleepSec  int    `yaml:"build_sleep_sec,omitempty" json:"build_sleep_sec,omitempty"`
	DeploySleepSec int    `yaml:"deploy_sleep_sec,omitempty" json:"deploy_sleep_sec,omitempty"`
}

// AppsConfig is the root of apps.yaml.
type AppsConfig struct {
	Apps []App `yaml:"apps"`
}

// LoadApps reads the YAML file at path (e.g. config/apps.yaml) and returns the list of apps.
// Returns an error if the file cannot be read or YAML is invalid.
func LoadApps(path string) ([]App, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg AppsConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return cfg.Apps, nil
}

// SaveApps marshals the given apps to YAML and writes the file at path.
// Used by the server when creating, updating, or deleting apps via the API.
func SaveApps(path string, apps []App) error {
	cfg := AppsConfig{Apps: apps}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// EffectiveSteps returns app steps. If dynamic steps are not provided, it builds them from legacy fields.
func (a App) EffectiveSteps() []Step {
	if len(a.Steps) > 0 {
		out := make([]Step, 0, len(a.Steps))
		for i, s := range a.Steps {
			name := strings.TrimSpace(s.Name)
			if name == "" {
				name = "step-" + strconvItoa(i+1)
			}
			normalized := Step{
				Name:     name,
				Cmd:      strings.TrimSpace(s.Cmd),
				File:     strings.TrimSpace(s.File),
				Script:   strings.TrimSpace(s.Script),
				SleepSec: s.SleepSec,
			}
			if normalized.Kind() == "" {
				continue
			}
			out = append(out, normalized)
		}
		return out
	}
	out := make([]Step, 0, 3)
	if strings.TrimSpace(a.TestCmd) != "" {
		out = append(out, Step{Name: "test", Cmd: strings.TrimSpace(a.TestCmd), SleepSec: a.TestSleepSec})
	}
	if strings.TrimSpace(a.BuildCmd) != "" {
		out = append(out, Step{Name: "build", Cmd: strings.TrimSpace(a.BuildCmd), SleepSec: a.BuildSleepSec})
	}
	if strings.TrimSpace(a.DeployCmd) != "" {
		out = append(out, Step{Name: "deploy", Cmd: strings.TrimSpace(a.DeployCmd), SleepSec: a.DeploySleepSec})
	}
	return out
}

// NormalizeAppSteps writes effective steps back to app.Steps.
func NormalizeAppSteps(app App) App {
	app.Steps = app.EffectiveSteps()
	return app
}

func strconvItoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + (n % 10))}, digits...)
		n /= 10
	}
	return string(digits)
}

// Package config loads and saves application definitions from and to YAML.
// The main file is typically config/apps.yaml; its path is given at startup.
// Apps are also persisted when created, updated, or deleted via the API.
package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// App defines a single application in the CI/CD system.
// ID is unique and used in URLs and as the clone directory name under work/.
// Repo is the git clone URL; Branch defaults to "main" if empty.
// TestCmd and BuildCmd are required; DeployCmd is optional.
// TestSleepSec, BuildSleepSec, DeploySleepSec are optional: when > 0, the pipeline sleeps that many seconds after the corresponding step.
type App struct {
	ID            string `yaml:"id" json:"id"`
	Name          string `yaml:"name" json:"name"`
	Repo          string `yaml:"repo" json:"repo"`
	Branch        string `yaml:"branch" json:"branch"`
	BuildCmd      string `yaml:"build_cmd" json:"build_cmd"`
	TestCmd       string `yaml:"test_cmd" json:"test_cmd"`
	DeployCmd     string `yaml:"deploy_cmd" json:"deploy_cmd"`
	TestSleepSec  int    `yaml:"test_sleep_sec" json:"test_sleep_sec"`
	BuildSleepSec int    `yaml:"build_sleep_sec" json:"build_sleep_sec"`
	DeploySleepSec int   `yaml:"deploy_sleep_sec" json:"deploy_sleep_sec"`
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

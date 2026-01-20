package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	ConfigDir      = ".claude/mob"
	ConfigFileName = "config.json"
	CurrentFile    = "current.json"
)

// Config holds the mob-claude configuration
type Config struct {
	APIURL      string `json:"apiUrl"`
	TeamName    string `json:"teamName"`
	Model       string `json:"model"`
	MaxTurns    int    `json:"maxTurns"`
	SkipSummary bool   `json:"skipSummary"`
}

// CurrentSession holds the current mob session metadata
type CurrentSession struct {
	Branch      string `json:"branch"`
	RepoURL     string `json:"repoUrl"`
	StartedAt   string `json:"startedAt"`
	DriverName  string `json:"driverName"`
	WorkstreamID string `json:"workstreamId,omitempty"`
}

// DefaultConfig returns a config with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		APIURL:      "http://localhost:3000",
		TeamName:    "",
		Model:       "haiku",
		MaxTurns:    3,
		SkipSummary: false,
	}
}

// GetConfigDir returns the path to the config directory in the current project
func GetConfigDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(cwd, ConfigDir), nil
}

// EnsureConfigDir creates the config directory if it doesn't exist
func EnsureConfigDir() (string, error) {
	dir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

// Load reads the config from disk, or returns defaults if not found
func Load() (*Config, error) {
	dir, err := GetConfigDir()
	if err != nil {
		return DefaultConfig(), nil
	}

	configPath := filepath.Join(dir, ConfigFileName)
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, err
	}

	cfg := &Config{}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// Apply defaults for missing fields
	defaults := DefaultConfig()
	if cfg.Model == "" {
		cfg.Model = defaults.Model
	}
	if cfg.MaxTurns == 0 {
		cfg.MaxTurns = defaults.MaxTurns
	}
	if cfg.APIURL == "" {
		cfg.APIURL = defaults.APIURL
	}

	return cfg, nil
}

// Save writes the config to disk
func Save(cfg *Config) error {
	dir, err := EnsureConfigDir()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	configPath := filepath.Join(dir, ConfigFileName)
	return os.WriteFile(configPath, data, 0644)
}

// LoadCurrentSession reads the current session metadata
func LoadCurrentSession() (*CurrentSession, error) {
	dir, err := GetConfigDir()
	if err != nil {
		return nil, err
	}

	sessionPath := filepath.Join(dir, CurrentFile)
	data, err := os.ReadFile(sessionPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	session := &CurrentSession{}
	if err := json.Unmarshal(data, session); err != nil {
		return nil, err
	}

	return session, nil
}

// SaveCurrentSession writes the current session metadata
func SaveCurrentSession(session *CurrentSession) error {
	dir, err := EnsureConfigDir()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}

	sessionPath := filepath.Join(dir, CurrentFile)
	return os.WriteFile(sessionPath, data, 0644)
}

// ClearCurrentSession removes the current session file
func ClearCurrentSession() error {
	dir, err := GetConfigDir()
	if err != nil {
		return err
	}

	sessionPath := filepath.Join(dir, CurrentFile)
	err = os.Remove(sessionPath)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

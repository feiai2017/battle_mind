package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

const (
	defaultPort           = 8080
	defaultTimeoutSeconds = 30
	defaultLogFilePath    = "logs/server.log"
)

// Config holds the minimal runtime configuration for the service.
type Config struct {
	Server  ServerConfig  `json:"server"`
	Model   ModelConfig   `json:"model"`
	Logging LoggingConfig `json:"logging"`
}

type ServerConfig struct {
	Port int `json:"port"`
}

type ModelConfig struct {
	APIKey         string `json:"api_key"`
	BaseURL        string `json:"base_url"`
	Model          string `json:"model"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

type LoggingConfig struct {
	FilePath string `json:"file_path"`
}

// Load reads configuration from a JSON file and applies minimal defaults.
func Load(path string) (*Config, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("config path is empty")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("config file not found: %s", path)
		}
		return nil, fmt.Errorf("read config file %s: %w", path, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config json %s: %w", path, err)
	}

	applyDefaults(&cfg)
	if err := validate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.Server.Port <= 0 {
		cfg.Server.Port = defaultPort
	}
	if cfg.Model.TimeoutSeconds <= 0 {
		cfg.Model.TimeoutSeconds = defaultTimeoutSeconds
	}

	cfg.Model.APIKey = strings.TrimSpace(cfg.Model.APIKey)
	cfg.Model.BaseURL = strings.TrimSpace(cfg.Model.BaseURL)
	cfg.Model.Model = strings.TrimSpace(cfg.Model.Model)
	cfg.Logging.FilePath = strings.TrimSpace(cfg.Logging.FilePath)
	if cfg.Logging.FilePath == "" {
		cfg.Logging.FilePath = defaultLogFilePath
	}
}

func validate(cfg *Config) error {
	if cfg.Model.APIKey == "" {
		return fmt.Errorf("invalid config: model.api_key is required")
	}
	if cfg.Model.BaseURL == "" {
		return fmt.Errorf("invalid config: model.base_url is required")
	}
	if cfg.Model.Model == "" {
		return fmt.Errorf("invalid config: model.model is required")
	}
	return nil
}

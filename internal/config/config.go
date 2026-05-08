package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	DefaultBaseURL = "https://api.housecallpro.com"
	EnvAPIKey      = "HOUSECALL_PRO_API_KEY"
	EnvConfigPath  = "HCP_CONFIG"

	authModeAPIKey = "api_key"
	authModeOAuth  = "oauth"
)

type Config struct {
	BaseURL  string         `json:"base_url"`
	Auth     AuthConfig     `json:"auth"`
	Defaults DefaultsConfig `json:"defaults,omitempty"`
}

type AuthConfig struct {
	Mode   string `json:"mode"`
	APIKey string `json:"api_key,omitempty"`
}

type DefaultsConfig struct {
	CompanyID   string   `json:"company_id,omitempty"`
	LocationIDs []string `json:"location_ids,omitempty"`
}

func Default() Config {
	return Config{
		BaseURL: DefaultBaseURL,
		Auth: AuthConfig{
			Mode: authModeAPIKey,
		},
	}
}

func DefaultPath() (string, error) {
	if configured := strings.TrimSpace(os.Getenv(EnvConfigPath)); configured != "" {
		return configured, nil
	}

	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}

	return filepath.Join(dir, "hcp", "config.json"), nil
}

func Load(path string) (Config, error) {
	if strings.TrimSpace(path) == "" {
		resolved, err := DefaultPath()
		if err != nil {
			return Config{}, err
		}
		path = resolved
	}

	cfg := Default()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}

	cfg.ApplyDefaults()
	return cfg, nil
}

func Save(path string, cfg Config) error {
	if strings.TrimSpace(path) == "" {
		resolved, err := DefaultPath()
		if err != nil {
			return err
		}
		path = resolved
	}

	cfg.ApplyDefaults()

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write config %s: %w", path, err)
	}

	return nil
}

func (c *Config) ApplyDefaults() {
	if strings.TrimSpace(c.BaseURL) == "" {
		c.BaseURL = DefaultBaseURL
	}
	if strings.TrimSpace(c.Auth.Mode) == "" {
		c.Auth.Mode = authModeAPIKey
	}
}

func (c Config) APIKey() string {
	if key := strings.TrimSpace(os.Getenv(EnvAPIKey)); key != "" {
		return key
	}
	return strings.TrimSpace(c.Auth.APIKey)
}

func (c Config) AuthMode() string {
	switch strings.ToLower(strings.TrimSpace(c.Auth.Mode)) {
	case authModeOAuth:
		return authModeOAuth
	default:
		return authModeAPIKey
	}
}

func (c Config) Redacted() Config {
	c.Auth.APIKey = Mask(c.Auth.APIKey)
	return c
}

func Mask(secret string) string {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return ""
	}
	if len(secret) <= 4 {
		return "****"
	}
	if len(secret) <= 8 {
		return "****" + secret[len(secret)-2:]
	}
	return "****" + secret[len(secret)-4:]
}

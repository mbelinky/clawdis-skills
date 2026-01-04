package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type HomeAssistant struct {
	URL   string `yaml:"url"`
	Token string `yaml:"token"`
}

type Cloud struct {
	AccessID  string `yaml:"accessId"`
	AccessKey string `yaml:"accessKey"`
	Endpoint  string `yaml:"endpoint"`
	Region    string `yaml:"region"`
	Schema    string `yaml:"schema"`
	UserID    string `yaml:"userId"`
}

type Config struct {
	Backend       string        `yaml:"backend"`
	HomeAssistant HomeAssistant `yaml:"homeAssistant"`
	Cloud         Cloud         `yaml:"cloud"`
}

func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "tuya-hub", "config.yaml"), nil
}

func Load(path string) (*Config, error) {
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return nil, err
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return &Config{}, nil
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) ApplyEnv() {
	if v := strings.TrimSpace(os.Getenv("TUYA_BACKEND")); v != "" {
		c.Backend = v
	}
	if v := strings.TrimSpace(os.Getenv("TUYA_HA_URL")); v != "" {
		c.HomeAssistant.URL = v
	}
	if v := strings.TrimSpace(os.Getenv("TUYA_HA_TOKEN")); v != "" {
		c.HomeAssistant.Token = v
	}
	if v := strings.TrimSpace(os.Getenv("TUYA_CLOUD_ACCESS_ID")); v != "" {
		c.Cloud.AccessID = v
	}
	if v := strings.TrimSpace(os.Getenv("TUYA_CLOUD_ACCESS_KEY")); v != "" {
		c.Cloud.AccessKey = v
	}
	if v := strings.TrimSpace(os.Getenv("TUYA_CLOUD_ENDPOINT")); v != "" {
		c.Cloud.Endpoint = v
	}
	if v := strings.TrimSpace(os.Getenv("TUYA_CLOUD_REGION")); v != "" {
		c.Cloud.Region = v
	}
	if v := strings.TrimSpace(os.Getenv("TUYA_CLOUD_SCHEMA")); v != "" {
		c.Cloud.Schema = v
	}
	if v := strings.TrimSpace(os.Getenv("TUYA_CLOUD_USER_ID")); v != "" {
		c.Cloud.UserID = v
	}
}

func (c *Config) BackendOr(defaultBackend string) string {
	if strings.TrimSpace(c.Backend) == "" {
		return defaultBackend
	}
	return c.Backend
}

func (c *Config) Validate(backend string) error {
	switch backend {
	case "ha":
		if strings.TrimSpace(c.HomeAssistant.URL) == "" {
			return errors.New("home assistant url missing (set homeAssistant.url or TUYA_HA_URL)")
		}
		if strings.TrimSpace(c.HomeAssistant.Token) == "" {
			return errors.New("home assistant token missing (set homeAssistant.token or TUYA_HA_TOKEN)")
		}
	case "cloud":
		if strings.TrimSpace(c.Cloud.AccessID) == "" || strings.TrimSpace(c.Cloud.AccessKey) == "" || strings.TrimSpace(c.Cloud.Endpoint) == "" {
			return errors.New("tuya cloud credentials missing (set cloud.accessId, cloud.accessKey, cloud.endpoint)")
		}
	default:
		return errors.New("unknown backend: " + backend)
	}
	return nil
}

func Save(path string, cfg *Config) (string, error) {
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return "", err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

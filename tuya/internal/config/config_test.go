package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadConfig(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "config.yaml")

	cfg := &Config{
		Backend: "cloud",
		Cloud: Cloud{
			AccessID:  "id",
			AccessKey: "key",
			Endpoint:  "https://openapi.tuyaus.com",
			Region:    "us",
			Schema:    "smartlife",
			UserID:    "uid",
		},
	}

	if _, err := Save(path, cfg); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if loaded.Backend != "cloud" {
		t.Fatalf("expected backend cloud, got %q", loaded.Backend)
	}
	if loaded.Cloud.UserID != "uid" {
		t.Fatalf("expected userId uid, got %q", loaded.Cloud.UserID)
	}
	if loaded.Cloud.Schema != "smartlife" {
		t.Fatalf("expected schema smartlife, got %q", loaded.Cloud.Schema)
	}
}

func TestApplyEnv(t *testing.T) {
	os.Setenv("TUYA_BACKEND", "cloud")
	os.Setenv("TUYA_CLOUD_SCHEMA", "smartlife")
	os.Setenv("TUYA_CLOUD_USER_ID", "uid")
	defer os.Unsetenv("TUYA_BACKEND")
	defer os.Unsetenv("TUYA_CLOUD_SCHEMA")
	defer os.Unsetenv("TUYA_CLOUD_USER_ID")

	cfg := &Config{}
	cfg.ApplyEnv()
	if cfg.Backend != "cloud" {
		t.Fatalf("expected backend cloud, got %q", cfg.Backend)
	}
	if cfg.Cloud.Schema != "smartlife" {
		t.Fatalf("expected schema smartlife, got %q", cfg.Cloud.Schema)
	}
	if cfg.Cloud.UserID != "uid" {
		t.Fatalf("expected userId uid, got %q", cfg.Cloud.UserID)
	}
}

package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shreyansh-shankar/getitback/internal/config"
)

func TestLoad_CreatesDefaultConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	_, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	configDir := filepath.Join(tmpDir, config.DefaultDir)
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		t.Error("expected config directory to be created")
	}

	configFile := filepath.Join(configDir, config.ConfigFile)
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		t.Error("expected config file to be created")
	}
}

func TestLoad_ReadsExistingConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	configDir := filepath.Join(tmpDir, config.DefaultDir)
	os.MkdirAll(configDir, 0700)

	customBackupPath := filepath.Join(tmpDir, "backups")
	customKeyPath := filepath.Join(tmpDir, "keys", "key.txt")

	cfgData := "storage:\n  path: " + customBackupPath + "\nencryption:\n  enabled: true\n  key_path: " + customKeyPath + "\n"
	os.WriteFile(filepath.Join(configDir, config.ConfigFile), []byte(cfgData), 0600)

	result, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if result.Storage.Path != customBackupPath {
		t.Errorf("expected storage path %s, got %s", customBackupPath, result.Storage.Path)
	}
	if !result.Encryption.Enabled {
		t.Error("expected encryption to be enabled")
	}
	if result.Encryption.KeyPath != customKeyPath {
		t.Errorf("expected key path %s, got %s", customKeyPath, result.Encryption.KeyPath)
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	configDir := filepath.Join(tmpDir, config.DefaultDir)
	os.MkdirAll(configDir, 0700)

	os.WriteFile(filepath.Join(configDir, config.ConfigFile), []byte("invalid: yaml: \n  broken"), 0600)

	_, err := config.Load()
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

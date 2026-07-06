package config

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

const (
	DefaultDir  = ".getitback"
	ConfigFile  = "config.yaml"
	BackupsDir  = "backups"
	KeyFile     = "key.txt"
)

type Config struct {
	Storage    StorageConfig    `mapstructure:"storage"`
	Encryption EncryptionConfig `mapstructure:"encryption"`
}

type StorageConfig struct {
	Path string `mapstructure:"path"`
}

type EncryptionConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	KeyPath string `mapstructure:"key_path"`
}

func Load() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	getitbackDir := filepath.Join(home, DefaultDir)
	if err := os.MkdirAll(getitbackDir, 0700); err != nil {
		return nil, err
	}

	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(getitbackDir)

	v.SetDefault("storage.path", filepath.Join(getitbackDir, BackupsDir))
	v.SetDefault("encryption.enabled", false)
	v.SetDefault("encryption.key_path", filepath.Join(getitbackDir, KeyFile))

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			if err := v.SafeWriteConfig(); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	if err := os.MkdirAll(cfg.Storage.Path, 0700); err != nil {
		return nil, err
	}

	return &cfg, nil
}

package config

import (
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server    ServerConfig    `mapstructure:"server" json:"server"`
	Reflector ReflectorConfig `mapstructure:"reflector" json:"reflector"`
	Logging   LoggingConfig   `mapstructure:"logging" json:"logging"`
	Audio     AudioConfig     `mapstructure:"audio" json:"audio"`
	Callbook  CallbookConfig  `mapstructure:"callbook" json:"callbook"`
}

type CallbookConfig struct {
	QRZUsername string `mapstructure:"qrz_username" json:"qrz_username"`
	QRZPassword string `mapstructure:"qrz_password" json:"qrz_password"`
}

type ServerConfig struct {
	Addr   string `mapstructure:"addr" json:"addr"`
	NNGURL string `mapstructure:"nng_url" json:"nng_url"`
	DBPath string `mapstructure:"db_path" json:"db_path"`
}

type ReflectorConfig struct {
	Name        string            `mapstructure:"name" json:"name"`
	Description string            `mapstructure:"description" json:"description"`
	Modules     map[string]string `mapstructure:"modules" json:"modules"`
}

type LoggingConfig struct {
	Level      string `mapstructure:"level"`
	FilePath   string `mapstructure:"file_path"`
	MaxSizeMB  int    `mapstructure:"max_size_mb"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAgeDays int    `mapstructure:"max_age_days"`
	Compress   bool   `mapstructure:"compress"`
	Console    bool   `mapstructure:"console"`
}

type AudioConfig struct {
	Enable bool   `mapstructure:"enable" json:"enable"`
	Path   string `mapstructure:"path" json:"path"`
}

func LoadConfig(path string) (*Config, error) {
	v := viper.New()

	// Defaults
	v.SetDefault("server.addr", ":8080")
	v.SetDefault("server.nng_url", "tcp://127.0.0.1:5555")
	v.SetDefault("server.db_path", "data/dashboard.db")
	v.SetDefault("reflector.name", "URFD Dashboard")
	v.SetDefault("reflector.description", "Universal Reflector Dashboard")
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.console", true)

	// Env vars
	v.SetEnvPrefix("URFD")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Config file
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		// It's okay if config file doesn't exist, we fallback to defaults/env
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var c Config
	if err := v.Unmarshal(&c); err != nil {
		return nil, err
	}

	return &c, nil
}

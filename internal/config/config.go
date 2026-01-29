package config

import (
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Server   ServerConfig `yaml:"server"`
	Pebble   PebbleConfig `yaml:"pebble"`
	Bitcoin  ChainConfig  `yaml:"bitcoin"`
	Litecoin ChainConfig  `yaml:"litecoin"`
}

// ServerConfig represents the HTTP server configuration
type ServerConfig struct {
	Port int    `yaml:"port"`
	Host string `yaml:"host"`
}

// PebbleConfig represents the Pebble database configuration
type PebbleConfig struct {
	Path string `yaml:"path"`
}

// ChainConfig represents the configuration for a blockchain node
type ChainConfig struct {
	Enabled      bool   `yaml:"enabled"`
	Host         string `yaml:"host"`
	User         string `yaml:"user"`
	Pass         string `yaml:"pass"`
	Cert         string `yaml:"cert"`
	DisableTLS   bool   `yaml:"disable_tls"`
	HTTPMode     bool   `yaml:"http_mode"`      // Use HTTP POST instead of WebSocket (for bitcoind/litecoind)
	PollInterval int    `yaml:"poll_interval"`  // Block polling interval in seconds (default: 10)
	StartHeight  int64  `yaml:"start_height"`
}

// Load loads configuration from a YAML file and environment variables
func Load(path string) (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Port: 8080,
			Host: "0.0.0.0",
		},
		Pebble: PebbleConfig{
			Path: "./data/pebble",
		},
	}

	// Load from YAML file if it exists
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("failed to read config file: %w", err)
			}
		} else {
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, fmt.Errorf("failed to parse config file: %w", err)
			}
		}
	}

	// Override with environment variables
	cfg.loadEnv()

	return cfg, nil
}

func (c *Config) loadEnv() {
	// Server config
	if port := os.Getenv("SERVER_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			c.Server.Port = p
		}
	}
	if host := os.Getenv("SERVER_HOST"); host != "" {
		c.Server.Host = host
	}

	// Pebble config
	if path := os.Getenv("PEBBLE_PATH"); path != "" {
		c.Pebble.Path = path
	}

	// Bitcoin config
	c.loadChainEnv(&c.Bitcoin, "BTC")

	// Litecoin config
	c.loadChainEnv(&c.Litecoin, "LTC")
}

func (c *Config) loadChainEnv(chain *ChainConfig, prefix string) {
	if enabled := os.Getenv(prefix + "_ENABLED"); enabled != "" {
		chain.Enabled = enabled == "true" || enabled == "1"
	}
	if host := os.Getenv(prefix + "_HOST"); host != "" {
		chain.Host = host
	}
	if user := os.Getenv(prefix + "_USER"); user != "" {
		chain.User = user
	}
	if pass := os.Getenv(prefix + "_PASS"); pass != "" {
		chain.Pass = pass
	}
	if cert := os.Getenv(prefix + "_CERT"); cert != "" {
		chain.Cert = cert
	}
	if disableTLS := os.Getenv(prefix + "_DISABLE_TLS"); disableTLS != "" {
		chain.DisableTLS = disableTLS == "true" || disableTLS == "1"
	}
	if httpMode := os.Getenv(prefix + "_HTTP_MODE"); httpMode != "" {
		chain.HTTPMode = httpMode == "true" || httpMode == "1"
	}
	if pollInterval := os.Getenv(prefix + "_POLL_INTERVAL"); pollInterval != "" {
		if p, err := strconv.Atoi(pollInterval); err == nil {
			chain.PollInterval = p
		}
	}
	if startHeight := os.Getenv(prefix + "_START_HEIGHT"); startHeight != "" {
		if h, err := strconv.ParseInt(startHeight, 10, 64); err == nil {
			chain.StartHeight = h
		}
	}
}

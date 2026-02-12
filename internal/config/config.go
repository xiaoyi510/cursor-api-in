package config

import (
	"encoding/json"
	"os"
	"sync"
)

type ModelRoute struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Enabled bool   `json:"enabled"`
}

type Provider struct {
	ID      string       `json:"id"`
	Name    string       `json:"name"`
	Type    string       `json:"type"`
	BaseURL string       `json:"base_url"`
	APIKey  string       `json:"api_key"`
	Weight  int          `json:"weight"`
	Timeout int          `json:"timeout"`
	Models  []ModelRoute `json:"models"`
}

type Config struct {
	Port          int        `json:"port"`
	APIKey        string     `json:"api_key"`
	AdminPassword string     `json:"admin_password"`
	Providers     []Provider `json:"providers"`
}

var (
	cfg   Config
	cfgMu sync.RWMutex
)

const configFile = "config.json"

func Load() error {
	cfgMu.Lock()
	defer cfgMu.Unlock()

	data, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			cfg = Config{Port: 3029, Providers: []Provider{}}
			return saveLocked()
		}
		return err
	}
	return json.Unmarshal(data, &cfg)
}

func saveLocked() error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configFile, data, 0644)
}

func Get() Config {
	cfgMu.RLock()
	defer cfgMu.RUnlock()
	c := cfg
	c.Providers = make([]Provider, len(cfg.Providers))
	copy(c.Providers, cfg.Providers)
	return c
}

func Set(c Config) error {
	cfgMu.Lock()
	cfg = c
	err := saveLocked()
	cfgMu.Unlock()
	return err
}

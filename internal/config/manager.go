package config

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type Manager struct {
	configPath string
	config     *Config
	viper      *viper.Viper
}

func NewManager() *Manager {
	home := getHomeDir()
	configPath := filepath.Join(home, ".liberida")
	os.MkdirAll(configPath, 755)

	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("toml")
	v.AddConfigPath(configPath)

	return &Manager{
		configPath: configPath,
		config:     DefaultConfig(),
		viper:      v,
	}
}

func (m *Manager) Load() error {
	m.viper.SetDefault("ollama_url", m.config.OllamaURL)
	m.viper.SetDefault("model", m.config.Model)
	m.viper.SetDefault("allowed_dir", m.config.AllowedDir)

	if err := m.viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileAlreadyExistsError); ok {
			return nil
		}
		return err
	}
	return m.viper.Unmarshal(m.config)
}

func (m *Manager) Save() error {
	m.viper.Set("ollama_url", m.config.OllamaURL)
	m.viper.Set("model", m.config.Model)
	m.viper.Set("allowed_dir", m.config.AllowedDir)

	configFile := filepath.Join(m.configPath, "config.toml")
	return m.viper.WriteConfigAs(configFile)
}

func (m *Manager) Get() *Config {
	return m.config
}

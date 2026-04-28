package config

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

const (
	defaultConfigName = "config"
	defaultConfigExt  = "toml"
	defaultConfig     = defaultConfigName + "." + defaultConfigExt
)

type Manager struct {
	configPath string
	config     *Config
	viper      *viper.Viper
	homeProv   HomeDirProvider
}

func NewManagerWithProvider(hp HomeDirProvider) *Manager {
	home := hp.GetHomeDir()
	configPath := filepath.Join(home, ".liberida")
	os.MkdirAll(configPath, 0755)

	v := viper.New()
	v.SetConfigName(defaultConfigName)
	v.SetConfigType(defaultConfigExt)
	v.AddConfigPath(configPath)

	return &Manager{
		configPath: configPath,
		config:     DefaultConfig(hp),
		viper:      v,
		homeProv:   hp,
	}
}

// NewManager creates a new configuration manager using the OS home directory.
func NewManager() *Manager {
	return NewManagerWithProvider(OSHomeDirProvider{})
}

func (m *Manager) Load() error {
	m.viper.SetDefault("provider", m.config.Provider)
	m.viper.SetDefault("ollama_url", m.config.OllamaURL)
	m.viper.SetDefault("model", m.config.Model)
	m.viper.SetDefault("context_size", m.config.ContextSize)
	m.viper.SetDefault("openai_api_key", m.config.OpenAIAPIKey)
	m.viper.SetDefault("anthropic_api_key", m.config.AnthropicAPIKey)
	m.viper.SetDefault("gemini_api_key", m.config.GeminiAPIKey)
	m.viper.SetDefault("auto_context", m.config.AutoContext)
	m.viper.SetDefault("auto_title", m.config.AutoTitle)

	// Try to read existing config file
	if err := m.viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// No config file found: create default one
			return m.Save() // Save defaults to disk
		}
		return err
	}

	// Unmashal the config file into the struct
	if err := m.viper.Unmarshal(m.config); err != nil {
		return err
	}

	return nil
}

func (m *Manager) Save() error {
	// Set all values in viper
	m.viper.Set("provider", m.config.Provider)
	m.viper.Set("ollama_url", m.config.OllamaURL)
	m.viper.Set("model", m.config.Model)
	m.viper.Set("context_size", m.config.ContextSize)
	m.viper.Set("openai_api_key", m.config.OpenAIAPIKey)
	m.viper.Set("anthropic_api_key", m.config.AnthropicAPIKey)
	m.viper.Set("gemini_api_key", m.config.GeminiAPIKey)
	m.viper.Set("auto_context", m.config.AutoContext)
	m.viper.Set("auto_title", m.config.AutoTitle)

	configFile := filepath.Join(m.configPath, defaultConfig)
	return m.viper.WriteConfigAs(configFile)
}

func (m *Manager) Get() *Config {
	return m.config
}

func (m *Manager) GetConfigPath() string {
	return filepath.Join(m.configPath, defaultConfig)
}

// UpdateFromMap updates the config with values from a map
// Useful for TUI or CLI flag updates
func (m *Manager) UpdateFromMap(updates map[string]interface{}) error {
	for key, value := range updates {
		m.viper.Set(key, value)
	}

	// Apply updates to config struct
	if err := m.viper.Unmarshal(m.config); err != nil {
		return err
	}

	return nil
}

// Reset restores the configuration to defaults
func (m *Manager) Reset() {
	m.config = DefaultConfig(m.homeProv)
}

// Exists checks if a config file exists on disk
func (m *Manager) Exists() bool {
	configFile := filepath.Join(m.configPath, "config.toml")
	_, err := os.Stat(configFile)
	return err == nil
}

// Delete removes the config file from disk
func (m *Manager) Delete() error {
	configFile := filepath.Join(m.configPath, "config.toml")
	err := os.Remove(configFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

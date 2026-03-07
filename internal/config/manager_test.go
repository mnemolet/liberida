package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewManager(t *testing.T) {
	// Use a temporary directory as fake home
	tmpDir := t.TempDir()
	provider := mockHomeDirProvider{dir: tmpDir}
	manager := NewManagerWithProvider(provider)

	if manager == nil {
		t.Fatal("Expected non-nil manager")
	}

	if manager.config == nil {
		t.Error("Expected config to be initialized")
	}

	if manager.viper == nil {
		t.Error("Expected viper to be initialized")
	}

	expectedPath := filepath.Join(tmpDir, ".liberida")
	if manager.configPath != expectedPath {
		t.Errorf("Expected config path %s, got %s", expectedPath, manager.configPath)
	}

	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Error("Expected config directory to be created")
	}
}

func TestManagerLoadWithNoConfig(t *testing.T) {
	tmpDir := t.TempDir()
	provider := mockHomeDirProvider{dir: tmpDir}
	manager := NewManagerWithProvider(provider)

	err := manager.Load()
	if err != nil {
		t.Errorf("Load() with no config file error = %v", err)
	}

	cfg := manager.Get()
	if cfg.OllamaURL != "http://localhost:11434" {
		t.Errorf("Expected default OllamaURL, got %s", cfg.OllamaURL)
	}
}

func TestManagerSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	provider := mockHomeDirProvider{dir: tmpDir}
	manager := NewManagerWithProvider(provider)

	// Modify config
	cfg := manager.Get()
	cfg.OllamaURL = "http://test:11434"
	cfg.Model = "test-model"
	cfg.ExecutionMode = ModeLocal
	cfg.AllowedDir = "/test/dir"
	cfg.ContainerName = "test-container"
	cfg.ContextSize = 20

	err := manager.Save()
	if err != nil {
		t.Errorf("Save() error = %v", err)
	}

	configFile := filepath.Join(tmpDir, ".liberida", "config.toml")
	if _, err := os.Stat(configFile); err != nil {
		t.Errorf("Config file not created: %v", err)
	}

	// Create new manager and load
	manager2 := NewManagerWithProvider(provider)
	err = manager2.Load()
	if err != nil {
		t.Errorf("Load() error = %v", err)
	}

	loadedCfg := manager2.Get()
	if loadedCfg.OllamaURL != "http://test:11434" {
		t.Errorf("Loaded OllamaURL = %s, want http://test:11434", loadedCfg.OllamaURL)
	}
	if loadedCfg.Model != "test-model" {
		t.Errorf("Loaded Model = %s, want test-model", loadedCfg.Model)
	}
	if loadedCfg.ExecutionMode != ModeLocal {
		t.Errorf("Loaded ExecutionMode = %s, want local", loadedCfg.ExecutionMode)
	}
	if loadedCfg.AllowedDir != "/test/dir" {
		t.Errorf("Loaded AllowedDir = %s, want /test/dir", loadedCfg.AllowedDir)
	}
	if loadedCfg.ContainerName != "test-container" {
		t.Errorf("Loaded ContainerName = %s, want test-container", loadedCfg.ContainerName)
	}
	if loadedCfg.ContextSize != 20 {
		t.Errorf("Loaded ContextSize = %d, want 20", loadedCfg.ContextSize)
	}
}

func TestManagerGetConfigPath(t *testing.T) {
	tmpDir := t.TempDir()
	provider := mockHomeDirProvider{dir: tmpDir}
	manager := NewManagerWithProvider(provider)

	path := manager.GetConfigPath()
	expected := filepath.Join(tmpDir, ".liberida", "config.toml")
	if path != expected {
		t.Errorf("GetConfigPath() = %s, want %s", path, expected)
	}
}

func TestManagerUpdateFromMap(t *testing.T) {
	tmpDir := t.TempDir()
	provider := mockHomeDirProvider{dir: tmpDir}
	manager := NewManagerWithProvider(provider)

	updates := map[string]interface{}{
		"ollama_url":     "http://updated:11434",
		"model":          "updated-model",
		"execution_mode": string(ModeDocker),
		"allowed_dir":    "/updated/dir",
		"container_name": "updated-container",
		"context_size":   30,
	}

	err := manager.UpdateFromMap(updates)
	if err != nil {
		t.Errorf("UpdateFromMap() error = %v", err)
	}

	cfg := manager.Get()
	if cfg.OllamaURL != "http://updated:11434" {
		t.Errorf("Updated OllamaURL = %s, want http://updated:11434", cfg.OllamaURL)
	}
	if cfg.Model != "updated-model" {
		t.Errorf("Updated Model = %s, want updated-model", cfg.Model)
	}
	if cfg.ExecutionMode != ModeDocker {
		t.Errorf("Updated ExecutionMode = %s, want docker", cfg.ExecutionMode)
	}
	if cfg.AllowedDir != "/updated/dir" {
		t.Errorf("Updated AllowedDir = %s, want /updated/dir", cfg.AllowedDir)
	}
	if cfg.ContainerName != "updated-container" {
		t.Errorf("Updated ContainerName = %s, want updated-container", cfg.ContainerName)
	}
	if cfg.ContextSize != 30 {
		t.Errorf("Updated ContextSize = %d, want 30", cfg.ContextSize)
	}
}

func TestManagerReset(t *testing.T) {
	tmpDir := t.TempDir()
	provider := mockHomeDirProvider{dir: tmpDir}
	manager := NewManagerWithProvider(provider)

	// Modify config
	cfg := manager.Get()
	cfg.OllamaURL = "http://modified:11434"
	cfg.Model = "modified"
	cfg.ExecutionMode = ModeDocker

	manager.Reset()

	// Get fresh config after reset
	resetCfg := manager.Get()

	defaultCfg := DefaultConfig(provider)
	if resetCfg.OllamaURL != defaultCfg.OllamaURL {
		t.Errorf("After reset OllamaURL = %s, want %s", resetCfg.OllamaURL, defaultCfg.OllamaURL)
	}
	if resetCfg.Model != defaultCfg.Model {
		t.Errorf("After reset Model = %s, want %s", resetCfg.Model, defaultCfg.Model)
	}
	if resetCfg.ExecutionMode != defaultCfg.ExecutionMode {
		t.Errorf("After reset ExecutionMode = %s, want %s", resetCfg.ExecutionMode, defaultCfg.ExecutionMode)
	}
}

func TestManagerExists(t *testing.T) {
	tmpDir := t.TempDir()
	provider := mockHomeDirProvider{dir: tmpDir}
	manager := NewManagerWithProvider(provider)

	if manager.Exists() {
		t.Error("Exists() = true, want false for new manager")
	}

	manager.Save()

	if !manager.Exists() {
		t.Error("Exists() = false, want true after save")
	}
}

func TestManagerDelete(t *testing.T) {
	tmpDir := t.TempDir()
	provider := mockHomeDirProvider{dir: tmpDir}
	manager := NewManagerWithProvider(provider)

	manager.Save()
	if !manager.Exists() {
		t.Fatal("Config should exist after save")
	}

	err := manager.Delete()
	if err != nil {
		t.Errorf("Delete() error = %v", err)
	}

	if manager.Exists() {
		t.Error("Config still exists after Delete()")
	}

	// Delete again should not error
	err = manager.Delete()
	if err != nil {
		t.Errorf("Delete() on non-existent file error = %v", err)
	}
}

func TestManagerLoadInvalidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	provider := mockHomeDirProvider{dir: tmpDir}
	manager := NewManagerWithProvider(provider)

	// Create invalid config file
	configDir := filepath.Join(tmpDir, ".liberida")
	os.MkdirAll(configDir, 0755)
	configFile := filepath.Join(configDir, "config.toml")
	err := os.WriteFile(configFile, []byte("invalid toml content ===="), 0644)
	if err != nil {
		t.Fatal(err)
	}

	err = manager.Load()
	if err == nil {
		t.Error("Load() with invalid config should error")
	}
}

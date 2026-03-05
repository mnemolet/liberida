package main

import (
	"os/user"
	"path/filepath"
	"strings"
)

type Config struct {
	OllamaURL  string
	Model      string
	AllowedDir string
}

func DefaultConfig() *Config {
	return &Config{
		OllamaURL:  "http://localhost:11434",
		Model:      "llama2",
		AllowedDir: "~/liberida-workspace",
	}
}

func getHomeDir() string {
	usr, err := user.Current()
	if err != nil {
		return "."
	}
	return usr.HomeDir
}

func ExpandPath(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}

	home := getHomeDir()

	if path == "~" {
		return home
	}

	return filepath.Join(home, path[2:])
}

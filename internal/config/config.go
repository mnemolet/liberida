package main

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

package config

// mockHomeDirProvider is a test helper that provides a fake home directory.
type mockHomeDirProvider struct {
	dir string
}

func (m mockHomeDirProvider) GetHomeDir() string {
	return m.dir
}

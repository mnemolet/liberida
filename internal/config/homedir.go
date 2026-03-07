package config

import (
	"os/user"
)

// HomeDirProvider abstracts getting the user's home directory.
type HomeDirProvider interface {
	GetHomeDir() string
}

// OSHomeDirProvider uses the os/user package to get the real home directory.
type OSHomeDirProvider struct{}

func (OSHomeDirProvider) GetHomeDir() string {
	usr, err := user.Current()
	if err != nil {
		return "."
	}
	return usr.HomeDir
}

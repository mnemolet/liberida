package stdinutil

import (
	"errors"
	"io"
	"os"
	"strings"
)

var ErrNoInput = errors.New("no input provided")

func IsPiped() bool {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) == 0
}

func ReadAll() (string, error) {
	if !IsPiped() {
		return "", ErrNoInput
	}

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func ReadAllTrimmed() (string, error) {
	data, err := ReadAll()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(data), nil
}
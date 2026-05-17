package stdinutil

import (
	"os"
	"testing"
)

func TestIsPiped(t *testing.T) {
	result := IsPiped()
	if result {
		t.Log("stdin is piped (expected in test context)")
	} else {
		t.Log("stdin is not piped (expected in test context)")
	}
}

func TestReadAll(t *testing.T) {
	result, err := ReadAll()
	if err == nil && result == "" {
		t.Log("No piped input, got empty string")
	}
}

func TestReadAllTrimmed(t *testing.T) {
	result, err := ReadAllTrimmed()
	if err == nil && result == "" {
		t.Log("No piped input, got empty string")
	}
}

func TestReadAllWithInput(t *testing.T) {
	origStat, _ := os.Stdin.Stat()
	if origStat.Mode()&os.ModeCharDevice != 0 {
		t.Skip("stdin is not piped in test environment")
	}

	data, err := ReadAll()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected some input data")
	}
}

func TestReadAllTrimmedWithInput(t *testing.T) {
	origStat, _ := os.Stdin.Stat()
	if origStat.Mode()&os.ModeCharDevice != 0 {
		t.Skip("stdin is not piped in test environment")
	}

	data, err := ReadAllTrimmed()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if data == "" {
		t.Error("expected non-empty trimmed input")
	}
}

func TestReadAllTrimmed_TrimsWhitespace(t *testing.T) {
	origStat, _ := os.Stdin.Stat()
	if origStat.Mode()&os.ModeCharDevice != 0 {
		t.Skip("stdin is not piped in test environment")
	}

	data, err := ReadAllTrimmed()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(data) > 0 {
		if data[0] == ' ' || data[len(data)-1] == ' ' {
			t.Error("expected trimmed whitespace")
		}
	}
}
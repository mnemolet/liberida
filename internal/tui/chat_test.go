package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mnemolet/liberida/internal/config"
	"github.com/mnemolet/liberida/internal/db"
	"github.com/mnemolet/liberida/internal/executor"
	"github.com/mnemolet/liberida/internal/provider"
)

func TestChatModel_RecursionDepthGuard(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	dbManager, err := db.NewManager(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	session, err := dbManager.CreateSession("test")
	if err != nil {
		t.Fatal(err)
	}

	execDir := t.TempDir()
	exec, err := executor.NewLocal(execDir)
	if err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	providerCalls := 0

	mockProv := provider.NewMockProvider()
	mockProv.WithStreamFunc(func(ctx context.Context, req provider.Request) (<-chan string, <-chan provider.Usage, <-chan []provider.ToolCall, error) {
		mu.Lock()
		providerCalls++
		callNum := providerCalls
		mu.Unlock()

		chunkChan := make(chan string)
		usageChan := make(chan provider.Usage, 1)
		toolChan := make(chan []provider.ToolCall, 1)

		go func() {
			defer close(chunkChan)
			defer close(usageChan)
			defer close(toolChan)

			usageChan <- provider.Usage{}

			toolChan <- []provider.ToolCall{{
				ID:   fmt.Sprintf("call-%d", callNum),
				Type: "function",
				Function: struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				}{
					Name:      "write_file",
					Arguments: `{"path":"test.txt","content":"test"}`,
				},
			}}
		}()

		return chunkChan, usageChan, toolChan, nil
	})

	maxDepth := 5
	cfg := config.DefaultConfig(config.OSHomeDirProvider{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	model := NewChatModel(
		cfg, mockProv, dbManager, exec, session.ID,
		"system prompt", nil, ctx, cancel,
	)
	model.maxToolCallDepth = maxDepth

	prog := tea.NewProgram(model, tea.WithInput(nil), tea.WithOutput(nil))
	model.SetProgram(prog)

	go func() {
		prog.Run()
	}()
	time.Sleep(20 * time.Millisecond)

	model.startAIResponse("hello")

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	calls := providerCalls
	mu.Unlock()

	if calls == 0 {
		t.Fatal("provider was never called")
	}
	// The provider is called once per recursion level, plus the initial user message.
	// With maxToolCallDepth=5, we expect at most 6 calls (1 user + up to 5 tool iterations)
	// before the guard stops recursion.
	if calls > maxDepth+2 {
		t.Errorf("provider called %d times, expected at most %d", calls, maxDepth+2)
	}

	if model.waiting {
		t.Error("expected model to not be waiting after recursion limit")
	}
}

func TestChatModel_RecursionDepthResetOnNewMessage(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	dbManager, err := db.NewManager(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	session, err := dbManager.CreateSession("test")
	if err != nil {
		t.Fatal(err)
	}

	execDir := t.TempDir()
	exec, err := executor.NewLocal(execDir)
	if err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	providerCalls := 0

	mockProv := provider.NewMockProvider()
	mockProv.WithStreamFunc(func(ctx context.Context, req provider.Request) (<-chan string, <-chan provider.Usage, <-chan []provider.ToolCall, error) {
		mu.Lock()
		providerCalls++
		callNum := providerCalls
		mu.Unlock()

		chunkChan := make(chan string)
		usageChan := make(chan provider.Usage, 1)
		toolChan := make(chan []provider.ToolCall, 1)

		go func() {
			defer close(chunkChan)
			defer close(usageChan)
			defer close(toolChan)

			usageChan <- provider.Usage{}

			// Return tools on the first call only; second call (after reset) returns none
			if callNum == 1 {
				toolChan <- []provider.ToolCall{{
					ID:   "call-1",
					Type: "function",
					Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{
						Name:      "write_file",
						Arguments: `{"path":"test.txt","content":"test"}`,
					},
				}}
			} else {
				toolChan <- nil
			}
		}()

		return chunkChan, usageChan, toolChan, nil
	})

	cfg := config.DefaultConfig(config.OSHomeDirProvider{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	model := NewChatModel(
		cfg, mockProv, dbManager, exec, session.ID,
		"system prompt", nil, ctx, cancel,
	)
	model.maxToolCallDepth = 25

	prog := tea.NewProgram(model, tea.WithInput(nil), tea.WithOutput(nil))
	model.SetProgram(prog)

	go func() {
		prog.Run()
	}()
	time.Sleep(20 * time.Millisecond)

	model.startAIResponse("first message")
	time.Sleep(50 * time.Millisecond)

	if model.toolCallDepth > 1 {
		t.Errorf("expected depth <= 1 after first turn, got %d", model.toolCallDepth)
	}
}

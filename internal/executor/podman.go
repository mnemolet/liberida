package executor

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

type PodmanExecutor struct {
	cli           *client.Client
	containerID   string
	containerName string
	workspaceDir  string
	mountPoint    string
}

func NewPodman(containerName, image, workspaceDir string) (*PodmanExecutor, error) {
	cli, err := client.New(
		client.WithHost("unix:///run/podman/podman.sock"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Podman: %w", err)
	}

	ctx := context.Background()

	if _, err := cli.Ping(ctx, client.PingOptions{}); err != nil {
		return nil, fmt.Errorf("podman not reachable: %w", err)
	}

	if containerName == "" {
		containerName = "ai-agent-workspace"
	}

	result, err := cli.ContainerList(ctx, client.ContainerListOptions{All: true})
	if err != nil {
		return nil, err
	}

	var containerID string
	for _, c := range result.Items {
		for _, name := range c.Names {
			if name == "/"+containerName || name == containerName {
				containerID = c.ID
				break
			}
		}
	}

	mountPoint := "/workspace"

	if containerID == "" {
		absWorkspace, err := filepath.Abs(workspaceDir)
		if err != nil {
			return nil, err
		}
		if err := os.MkdirAll(absWorkspace, 0755); err != nil {
			return nil, err
		}

		hostConfig := &container.HostConfig{
			Binds: []string{absWorkspace + ":" + mountPoint},
		}

		// Wrap configurations into the Options struct
		opts := client.ContainerCreateOptions{
			Name: containerName,
			Config: &container.Config{
				Image:      image,
				Cmd:        []string{"sleep", "infinity"},
				WorkingDir: mountPoint,
			},
			HostConfig:       hostConfig,
			NetworkingConfig: nil,
			Platform:         nil,
		}

		resp, err := cli.ContainerCreate(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to create container: %w", err)
		}

		if err != nil {
			return nil, fmt.Errorf("failed to create container: %w", err)
		}
		containerID = resp.ID
	}

	_, err = cli.ContainerStart(ctx, containerID, client.ContainerStartOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	return &PodmanExecutor{
		cli:           cli,
		containerID:   containerID,
		containerName: containerName,
		workspaceDir:  workspaceDir,
		mountPoint:    mountPoint,
	}, nil
}

// Close stops the container and shuts down the client.
func (p *PodmanExecutor) Close() error {
	timeout := 30

	stopOptions := client.ContainerStopOptions{
		Timeout: &timeout,
	}

	_, err := p.cli.ContainerStop(context.Background(), p.containerID, stopOptions)
	if err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	return p.cli.Close()
}

func (p *PodmanExecutor) RunCommand(ctx context.Context, command []string) (string, error) {
	execConfig := client.ExecCreateOptions{
		Cmd:          command,
		AttachStdout: true,
		AttachStderr: true,
		WorkingDir:   p.mountPoint,
	}

	execRes, err := p.cli.ExecCreate(ctx, p.containerID, execConfig)
	if err != nil {
		return "", fmt.Errorf("failed to create exec: %w", err)
	}

	resp, err := p.cli.ExecAttach(ctx, execRes.ID, client.ExecAttachOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to attach to exec: %w", err)
	}
	defer resp.Close()

	// Read output.
	var outBuf strings.Builder
	_, err = io.Copy(&outBuf, resp.Reader)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("failed to read exec output: %w", err)
	}

	inspect, err := p.cli.ExecInspect(ctx, execRes.ID, client.ExecInspectOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to inspect exec: %w", err)
	}

	if inspect.ExitCode != 0 {
		return outBuf.String(), fmt.Errorf("command exited with code %d", inspect.ExitCode)
	}

	return outBuf.String(), nil
}

// ListFiles uses host-native filepath walking for speed.
func (p *PodmanExecutor) ListFiles() ([]string, error) {
	var files []string

	// Walk the host directory directly.
	err := filepath.Walk(p.workspaceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			// Get path relative to the workspace root
			rel, err := filepath.Rel(p.workspaceDir, path)
			if err != nil {
				return err
			}
			files = append(files, rel)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list files on host: %w", err)
	}
	return files, nil
}

// WriteFile writes directly to the host-mounted directory.
func (p *PodmanExecutor) WriteFile(path string, data []byte) error {
	if filepath.IsAbs(path) {
		return fmt.Errorf("absolute paths are not allowed; use relative paths from workspace root")
	}

	// Resolve the actual path on the host
	fullHostPath := filepath.Join(p.workspaceDir, path)

	// Ensure sub-directories exist
	if err := os.MkdirAll(filepath.Dir(fullHostPath), 0755); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	// Write the file directly using standard Go os package
	if err := os.WriteFile(fullHostPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file to host: %w", err)
	}

	return nil
}

// ReadFile reads directly from the host-mounted directory.
func (p *PodmanExecutor) ReadFile(path string) ([]byte, error) {
	fullHostPath := filepath.Join(p.workspaceDir, path)
	return os.ReadFile(fullHostPath)
}

// DeleteFile removes a file from the host-mounted directory.
func (p *PodmanExecutor) DeleteFile(path string) error {
	// Safety check: Prevent absolute paths or directory traversal
	if filepath.IsAbs(path) || strings.Contains(path, "..") {
		return fmt.Errorf("invalid path: absolute paths or traversal not allowed")
	}

	// Resolve the actual path on the host
	fullHostPath := filepath.Join(p.workspaceDir, path)

	// Perform the deletion using native OS call
	if err := os.Remove(fullHostPath); err != nil {
		if os.IsNotExist(err) {
			// TODO: Decide if "file not found" should be an error or success
			return nil
		}
		return fmt.Errorf("failed to delete file on host: %w", err)
	}

	return nil
}

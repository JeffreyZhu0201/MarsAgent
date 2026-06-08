package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

// RunRequest 是 POST /api/sandbox/run 的请求体。
type RunRequest struct {
	Lang    string `json:"lang"` // python | node | go
	Code    string `json:"code"`
	Stdin   string `json:"stdin,omitempty"`
	Timeout int    `json:"timeout"` // seconds, default 15
}

// RunResult 是执行结果。
type RunResult struct {
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	ExitCode  int    `json:"exit_code"`
	Duration  int64  `json:"duration_ms"`
	Truncated bool   `json:"truncated"`
}

var imageMap = map[string]string{
	"python": "python:3.11-slim",
	"node":   "node:20-alpine",
	"go":     "golang:1.22-alpine",
}

// Scheduler manages Docker containers for code execution.
type Scheduler struct {
	cli *client.Client
}

// NewScheduler creates a new Sandbox Scheduler.
func NewScheduler() (*Scheduler, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("docker client: %w", err)
	}
	return &Scheduler{cli: cli}, nil
}

// Run executes code in an isolated Docker container.
func (s *Scheduler) Run(req RunRequest) (*RunResult, error) {
	if req.Timeout == 0 {
		req.Timeout = 15
	}
	img := imageMap[req.Lang]
	if img == "" {
		req.Lang = "python"
		img = imageMap[req.Lang]
	}

	cmd := commandFor(req)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(req.Timeout+5)*time.Second)
	defer cancel()

	pidsLimit := int64(64)
	hostConfig := &container.HostConfig{
		Resources: container.Resources{
			Memory:   256 * 1024 * 1024,
			NanoCPUs: int64(500 * 1e6),
			PidsLimit: &pidsLimit,
		},
		NetworkMode:    "none",
		ReadonlyRootfs: true,
		CapDrop:        []string{"ALL"},
		Tmpfs:          map[string]string{"/tmp": "rw,nosuid,size=64m"},
	}

	resp, err := s.cli.ContainerCreate(ctx, &container.Config{
		Image:        img,
		Cmd:          cmd,
		Tty:          false,
		AttachStdout: true,
		AttachStderr: true,
		OpenStdin:    req.Stdin != "",
		StdinOnce:    req.Stdin != "",
	}, hostConfig, nil, nil, "")
	if err != nil {
		return nil, fmt.Errorf("container create: %w", err)
	}
	defer s.cli.ContainerRemove(context.Background(), resp.ID, container.RemoveOptions{Force: true})

	start := time.Now()
	if err := s.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return &RunResult{ExitCode: -1, Stderr: err.Error()}, nil
	}

	statusCh, errCh := s.cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	var exitCode int
	select {
	case <-ctx.Done():
		return &RunResult{ExitCode: -1, Stderr: "timeout", Duration: time.Since(start).Milliseconds()}, nil
	case err := <-errCh:
		return nil, fmt.Errorf("container wait: %w", err)
	case status := <-statusCh:
		exitCode = int(status.StatusCode)
	}

	logs, err := s.cli.ContainerLogs(context.Background(), resp.ID, container.LogsOptions{ShowStdout: true, ShowStderr: true})
	if err != nil {
		return nil, fmt.Errorf("container logs: %w", err)
	}
	defer logs.Close()

	var stdout, stderr bytes.Buffer
	if _, err := stdcopy.StdCopy(&stdout, &stderr, logs); err != nil {
		buf, _ := io.ReadAll(logs)
		stdout.Write(buf)
	}

	return &RunResult{
		ExitCode: exitCode,
		Stdout:   truncate(stdout.String()),
		Stderr:   truncate(stderr.String()),
		Duration: time.Since(start).Milliseconds(),
	}, nil
}

func commandFor(req RunRequest) []string {
	switch req.Lang {
	case "node":
		return []string{"node", "-e", req.Code}
	case "go":
		return []string{"sh", "-c", "cat > /tmp/main.go <<'EOF'\n" + req.Code + "\nEOF\ncd /tmp && go run main.go"}
	default:
		return []string{"python3", "-c", req.Code}
	}
}

func truncate(s string) string {
	const maxOutput = 64 * 1024
	if len(s) <= maxOutput {
		return s
	}
	return strings.TrimRight(s[:maxOutput], "\x00")
}

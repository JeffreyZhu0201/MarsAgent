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

const (
	MaxCodeBytes   = 64 * 1024
	MaxOutputBytes = 64 * 1024
	MaxTimeoutSec  = 30
)

var imageMap = map[string]string{
	"python": "python:3.11-slim",
	"node":   "node:20-alpine",
	"go":     "golang:1.22-alpine",
}

func ValidateRequest(req RunRequest) error {
	if strings.TrimSpace(req.Code) == "" {
		return fmt.Errorf("code is required")
	}
	if len(req.Code) > MaxCodeBytes {
		return fmt.Errorf("code exceeds %d bytes", MaxCodeBytes)
	}
	if req.Timeout < 0 || req.Timeout > MaxTimeoutSec {
		return fmt.Errorf("timeout must be between 0 and %d seconds", MaxTimeoutSec)
	}
	if req.Lang != "" && imageMap[req.Lang] == "" {
		return fmt.Errorf("unsupported lang %q", req.Lang)
	}
	return nil
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
			Memory:    256 * 1024 * 1024,
			NanoCPUs:  int64(500 * 1e6),
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
		User:         "65534:65534",
		Env: []string{
			"HOME=/tmp",
			"GOCACHE=/tmp/go-cache",
			"GOMODCACHE=/tmp/go/pkg/mod",
			"TMPDIR=/tmp",
			"USER_CODE=" + req.Code,
			"USER_STDIN=" + req.Stdin,
		},
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

	stdout := newLimitedBuffer(MaxOutputBytes)
	stderr := newLimitedBuffer(MaxOutputBytes)
	if _, err := stdcopy.StdCopy(stdout, stderr, logs); err != nil {
		_, _ = io.Copy(stdout, logs)
	}

	return &RunResult{
		ExitCode:  exitCode,
		Stdout:    stdout.String(),
		Stderr:    stderr.String(),
		Duration:  time.Since(start).Milliseconds(),
		Truncated: stdout.Truncated() || stderr.Truncated(),
	}, nil
}

func commandFor(req RunRequest) []string {
	pipe := ""
	if req.Stdin != "" {
		pipe = "printf '%s' \"$USER_STDIN\" | "
	}
	switch req.Lang {
	case "node":
		return []string{"sh", "-c", pipe + "node -e \"$USER_CODE\""}
	case "go":
		return []string{"sh", "-c", "mkdir -p /tmp/go-cache /tmp/go/pkg/mod && printf '%s' \"$USER_CODE\" > /tmp/main.go && cd /tmp && " + pipe + "go run main.go"}
	default:
		return []string{"sh", "-c", pipe + "python3 -c \"$USER_CODE\""}
	}
}

type limitedBuffer struct {
	buf       bytes.Buffer
	max       int
	truncated bool
}

func newLimitedBuffer(max int) *limitedBuffer {
	return &limitedBuffer{max: max}
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	written := len(p)
	remaining := b.max - b.buf.Len()
	if remaining <= 0 {
		b.truncated = true
		return written, nil
	}
	if len(p) > remaining {
		_, _ = b.buf.Write(p[:remaining])
		b.truncated = true
		return written, nil
	}
	_, _ = b.buf.Write(p)
	return written, nil
}

func (b *limitedBuffer) String() string {
	return strings.TrimRight(b.buf.String(), "\x00")
}

func (b *limitedBuffer) Truncated() bool {
	return b.truncated
}

package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// RunRequest 是 POST /api/sandbox/run 的请求体。
type RunRequest struct {
	Lang    string `json:"lang"`    // python | node | go
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

var extMap = map[string]string{
	"python": "py",
	"node":   "js",
	"go":     "go",
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
	img, ok := imageMap[req.Lang]
	if !ok {
		img = "python:3.11-slim"
	}
	ext := extMap[req.Lang]
	if ext == "" {
		ext = "py"
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(req.Timeout+10)*time.Second)
	defer cancel()

	pidsLimit := int64(64)
	// Create a tmpfs-mounted container for isolation
	hostConfig := &container.HostConfig{
		Resources: container.Resources{
			Memory:   256 * 1024 * 1024, // 256MB
			NanoCPUs: int64(500 * 1e6),  // 0.5 CPU
			PidsLimit: &pidsLimit,
		},
		NetworkMode:    "none",
		ReadonlyRootfs: true,
		CapDrop:        []string{"ALL"},
		Tmpfs:          map[string]string{"/tmp": "rw,noexec,nosuid,size=64m"},
	}

	bootstrap := fmt.Sprintf("cat > /tmp/main.%s << 'EOF'\n%s\nEOF", ext, req.Code)
	execCmd := []string{"sh", "-c", fmt.Sprintf("timeout %d python3 /tmp/main.%s", req.Timeout, ext)}
	if req.Lang == "node" {
		execCmd = []string{"sh", "-c", fmt.Sprintf("timeout %d node /tmp/main.%s", req.Timeout, ext)}
	} else if req.Lang == "go" {
		execCmd = []string{"sh", "-c", fmt.Sprintf("timeout %d sh -c 'cd /tmp && go run main.%s'", req.Timeout, ext)}
	}

	resp, err := s.cli.ContainerCreate(ctx, &container.Config{
		Image:        img,
		Cmd:          []string{"sh", "-c", bootstrap},
		Tty:          false,
		AttachStdout: true,
		AttachStderr: true,
	}, hostConfig, nil, nil, "")
	if err != nil {
		return nil, fmt.Errorf("container create: %w", err)
	}
	defer s.cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})

	// Start the bootstrap container
	if err := s.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return &RunResult{ExitCode: -1, Stderr: err.Error()}, nil
	}

	// Wait for bootstrap to complete
	waitCh, errCh := s.cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case <-ctx.Done():
		return &RunResult{ExitCode: -1, Stderr: "timeout"}, nil
	case err := <-errCh:
		return nil, fmt.Errorf("container wait: %w", err)
	case status := <-waitCh:
		if status.StatusCode != 0 {
			return &RunResult{ExitCode: int(status.StatusCode)}, nil
		}
	}

	// Create exec to run the actual code
	execResp, err := s.cli.ContainerExecCreate(ctx, resp.ID, container.ExecOptions{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          execCmd,
	})
	if err != nil {
		return nil, fmt.Errorf("exec create: %w", err)
	}

	execStartCh, err := s.cli.ContainerExecAttach(ctx, execResp.ID, container.ExecAttachOptions{})
	if err != nil {
		return nil, fmt.Errorf("exec attach: %w", err)
	}
	defer execStartCh.Close()

	var stdout, stderr bytes.Buffer
	doneCh := make(chan struct{})
	go func() {
		_, _ = io.Copy(&stdout, execStartCh.Reader)
		close(doneCh)
	}()

	// Start the exec
	if err := s.cli.ContainerExecStart(ctx, execResp.ID, container.ExecStartOptions{}); err != nil {
		return nil, fmt.Errorf("exec start: %w", err)
	}

	// Poll for exec completion
	for {
		inspect, err := s.cli.ContainerExecInspect(ctx, execResp.ID)
		if err != nil {
			return nil, fmt.Errorf("exec inspect: %w", err)
		}
		if !inspect.Running {
			select {
			case <-doneCh:
			case <-time.After(500 * time.Millisecond):
			}
			return &RunResult{
				ExitCode: int(inspect.ExitCode),
				Stdout:   stdout.String(),
				Stderr:   stderr.String(),
				Duration: int64(req.Timeout * 1000),
			}, nil
		}
		select {
		case <-ctx.Done():
			return &RunResult{ExitCode: -1, Stderr: "timeout"}, nil
		case <-time.After(100 * time.Millisecond):
		}
	}
}
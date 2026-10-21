package runner

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
)

const (
	ContainerPort = "8080/tcp"
	DefaultImage  = "mock-exchange:latest"
)

type RunDetails struct {
	RunID      string `json:"run_id"`
	TeamID     string `json:"team_id"`
	ExposePort int    `json:"expose_port"`
	ImageURL   string `json:"image_url"`
}

type SandboxRunner struct {
	cli *client.Client
}

// NewSandboxRunner initializes a Docker client.
// Create one runner instance and reuse it for the lifecycle of a run.
func NewSandboxRunner() (*SandboxRunner, error) {
	cli, err := client.New(client.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("initialize Docker client: %w", err)
	}

	return &SandboxRunner{
		cli: cli,
	}, nil
}

// Close releases the Docker client's resources.
func (sr *SandboxRunner) Close() {
	if sr != nil && sr.cli != nil {
		sr.cli.Close()
	}
}

// StartSandbox creates and starts a contestant container.
// It returns the Docker container ID.
func (sr *SandboxRunner) StartSandbox(
	ctx context.Context,
	rd *RunDetails,
) (string, error) {
	if sr == nil || sr.cli == nil {
		return "", fmt.Errorf("sandbox runner is not initialized")
	}

	if rd == nil {
		return "", fmt.Errorf("run details cannot be nil")
	}

	image := strings.TrimSpace(rd.ImageURL)
	if image == "" {
		image = DefaultImage
	}

	// Generate a unique container name for each run.
	containerName := fmt.Sprintf(
		"sandbox-team-%s-%s",
		safeName(rd.TeamID),
		safeName(rd.RunID),
	)

	// Host-side configuration.
	hostConfig := &container.HostConfig{
		PortBindings: network.PortMap{
			network.MustParsePort(ContainerPort): []network.PortBinding{
				{
					HostIP:   netip.MustParseAddr("127.0.0.1"),
					HostPort: strconv.Itoa(rd.ExposePort),
				},
			},
		},

		Resources: container.Resources{
			CPUQuota:   200000,
			CPUPeriod:  100000,
			Memory:     1024 * 1024 * 1024,
			CpusetCpus: "0,1",
		},

		AutoRemove: true,
	}

	// Container configuration.
	config := &container.Config{
		ExposedPorts: network.PortSet{
			network.MustParsePort(ContainerPort): struct{}{},
		},
	}
	// Create the container.
	resp, err := sr.cli.ContainerCreate(
		ctx,
		client.ContainerCreateOptions{
			Image:      image,
			Config:     config,
			HostConfig: hostConfig,
			Name:       containerName,
		},
	)
	if err != nil {
		return "", fmt.Errorf("create sandbox container: %w", err)
	}

	// Start the container.
	_, err = sr.cli.ContainerStart(
		ctx,
		resp.ID,
		client.ContainerStartOptions{},
	)
	if err != nil {
		// Best-effort cleanup if the container cannot start.
		_, _ = sr.cli.ContainerRemove(
			context.Background(),
			resp.ID,
			client.ContainerRemoveOptions{
				Force: true,
			},
		)

		return "", fmt.Errorf("start sandbox container: %w", err)
	}

	fmt.Printf(
		"[Sandbox] Started container %s (ID: %s), host port %s\n",
		containerName,
		resp.ID,
		rd.ExposePort,
	)

	return resp.ID, nil
}

// WaitForReady waits until a TCP connection can be established.
// This confirms network reachability, not application-level health.
func (sr *SandboxRunner) WaitForReady(
	ctx context.Context,
	host string,
	port int,
	timeout time.Duration,
) error {
	if timeout <= 0 {
		return fmt.Errorf("readiness timeout must be positive")
	}

	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	target := net.JoinHostPort(host, strconv.Itoa(port))
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	var lastErr error

	for {
		dialer := net.Dialer{
			Timeout: 200 * time.Millisecond,
		}

		conn, err := dialer.DialContext(waitCtx, "tcp", target)
		if err == nil {
			_ = conn.Close()
			fmt.Printf("[Sandbox] Service reachable at %s\n", target)
			return nil
		}

		lastErr = err

		select {
		case <-waitCtx.Done():
			return fmt.Errorf(
				"service at %s did not become ready within %s: %w (last connection error: %v)",
				target,
				timeout,
				waitCtx.Err(),
				lastErr,
			)

		case <-ticker.C:
			// Retry until ready or the context expires.
		}
	}
}

// StopSandbox stops and removes a container.
// Pass the container ID returned by StartSandbox.
func (sr *SandboxRunner) StopSandbox(
	ctx context.Context,
	containerID string,
) error {
	if sr == nil || sr.cli == nil {
		return fmt.Errorf("sandbox runner is not initialized")
	}

	if strings.TrimSpace(containerID) == "" {
		return fmt.Errorf("container ID is required")
	}

	timeout := 5

	_, stopErr := sr.cli.ContainerStop(
		ctx,
		containerID,
		client.ContainerStopOptions{
			Timeout: &timeout,
		},
	)

	// Force removal also handles containers that have already stopped.
	_, removeErr := sr.cli.ContainerRemove(
		ctx,
		containerID,
		client.ContainerRemoveOptions{
			Force: true,
		},
	)

	if stopErr != nil && removeErr != nil {
		return fmt.Errorf(
			"stop container failed: %v; remove container failed: %w",
			stopErr,
			removeErr,
		)
	}

	if removeErr != nil {
		return fmt.Errorf("remove sandbox container: %w", removeErr)
	}

	fmt.Printf("[Sandbox] Stopped and removed container %s\n", shortID(containerID))
	return nil
}

func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func safeName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))

	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}

	result := strings.Trim(b.String(), "-_")
	if result == "" {
		return "unknown"
	}

	return result
}

package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/netip"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
)

const (
	ContainerImage = "mock-exchange:latest"
	HostPort       = "8080"
	ContainerPort  = "8080/tcp"
)

type SandboxRunner struct {
	cli *client.Client
}

func NewSandboxRunner() (*SandboxRunner, error) {
	cli, err := client.New(client.FromEnv)
	if err != nil {
		return nil, err
	}

	return &SandboxRunner{
		cli: cli,
	}, nil
}

func (sr *SandboxRunner) StartSandbox(
	ctx context.Context,
	teamID string,
) (string, error) {

	containerName := fmt.Sprintf(
		"sandbox-team-%s",
		teamID,
	)

	// Remove existing container.
	_, _ = sr.cli.ContainerRemove(
		ctx,
		containerName,
		client.ContainerRemoveOptions{
			Force: true,
		},
	)

	// Host configuration.
	// Host-side configuration.
	hostConfig := &container.HostConfig{
		PortBindings: network.PortMap{
			network.MustParsePort("8080/tcp"): []network.PortBinding{
				{
					HostIP:   netip.MustParseAddr("127.0.0.1"),
					HostPort: HostPort,
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
			network.MustParsePort("8080/tcp"): struct{}{},
		},
	}

	// Create container.
	resp, err := sr.cli.ContainerCreate(
		ctx,
		client.ContainerCreateOptions{
			Image:      ContainerImage,
			Config:     config,
			HostConfig: hostConfig,
			Name:       containerName,
		},
	)

	if err != nil {
		return "", fmt.Errorf(
			"failed to create sandbox container: %w",
			err,
		)
	}

	_, err = sr.cli.ContainerStart(
		ctx,
		resp.ID,
		client.ContainerStartOptions{},
	)
	if err != nil {
		return "", fmt.Errorf(
			"failed to start sandbox container: %w",
			err,
		)
	}

	log.Printf(
		"[Sandbox] Created and started container %s (ID: %s)",
		containerName,
		resp.ID[:12],
	)

	return resp.ID, nil
}

func (sr *SandboxRunner) WaitForReady(
	host string,
	port string,
	timeout time.Duration,
) error {

	deadline := time.Now().Add(timeout)

	target := net.JoinHostPort(host, port)

	for time.Now().Before(deadline) {

		conn, err := net.DialTimeout(
			"tcp",
			target,
			200*time.Millisecond,
		)

		if err == nil {
			_ = conn.Close()
			return nil
		}

		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf(
		"timeout waiting for container service at %s",
		target,
	)
}

func (sr *SandboxRunner) StopSandbox(
	ctx context.Context,
	containerID string,
) error {

	timeout := 0

	_, err := sr.cli.ContainerStop(
		ctx,
		containerID,
		client.ContainerStopOptions{
			Timeout: &timeout,
		},
	)

	if err != nil {
		return fmt.Errorf(
			"failed to stop sandbox: %w",
			err,
		)
	}

	return nil
}

func main() {

	ctx := context.Background()

	runner, err := NewSandboxRunner()
	if err != nil {
		log.Fatalf(
			"Failed to initialize Docker SDK: %v",
			err,
		)
	}

	log.Println(
		"--- Starting Step 3: Sandboxing Workflow ---",
	)

	containerID, err := runner.StartSandbox(
		ctx,
		"team-alpha",
	)

	if err != nil {
		log.Fatalf(
			"Sandbox Error: %v",
			err,
		)
	}

	log.Println("Probing container health...")

	if err := runner.WaitForReady(
		"127.0.0.1",
		HostPort,
		5*time.Second,
	); err != nil {

		_ = runner.StopSandbox(
			ctx,
			containerID,
		)

		log.Fatalf(
			"Healthcheck failed: %v",
			err,
		)
	}

	log.Println(
		"Container is HEALTHY and listening on ws://127.0.0.1:8080/ws",
	)

	log.Println(
		"Sandbox active. Press Ctrl+C to destroy sandbox...",
	)

	select {}
}

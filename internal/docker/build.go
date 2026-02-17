// Package docker wraps Docker CLI commands used for building and pushing images.
package docker

import (
	"fmt"
	"os/exec"
	"strings"
)

// CheckAvailable checks if Docker CLI is installed and running.
func CheckAvailable() error {
	out, err := exec.Command("docker", "version", "--format", "{{.Server.Version}}").Output()
	if err != nil {
		return fmt.Errorf("docker not available: %w", err)
	}
	version := strings.TrimSpace(string(out))
	if version == "" {
		return fmt.Errorf("docker daemon not running")
	}
	return nil
}

// Build builds a Docker image from a Dockerfile.
func Build(tag, dockerfile, context string) error {
	cmd := exec.Command("docker", "build", "-t", tag, "-f", dockerfile, context)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker build failed: %s\n%s", err, string(out))
	}
	return nil
}

// TagAndPush tags a local image and pushes it to a registry.
func TagAndPush(localTag, remoteTag string) error {
	if out, err := exec.Command("docker", "tag", localTag, remoteTag).CombinedOutput(); err != nil {
		return fmt.Errorf("docker tag failed: %s\n%s", err, string(out))
	}
	if out, err := exec.Command("docker", "push", remoteTag).CombinedOutput(); err != nil {
		return fmt.Errorf("docker push failed: %s\n%s", err, string(out))
	}
	return nil
}

// ComposeDown runs docker compose down in the specified directory.
func ComposeDown(dir string) error {
	cmd := exec.Command("docker", "compose", "down")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker compose down failed: %s\n%s", err, string(out))
	}
	return nil
}

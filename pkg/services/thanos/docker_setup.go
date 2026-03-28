package thanos

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/tokamak-network/trh-backend/internal/logger"
	"go.uber.org/zap"
)

const (
	dockerVersion        = "27.5.1"
	dockerComposeVersion = "v2.32.4"
)

// ensureDockerTools installs Docker CLI and Docker Compose plugin if not already present.
// This is only needed for local deployments (Docker-in-Docker pattern).
func ensureDockerTools() error {
	if err := ensureDockerCLI(); err != nil {
		return fmt.Errorf("failed to install docker CLI: %w", err)
	}
	if err := ensureDockerCompose(); err != nil {
		return fmt.Errorf("failed to install docker compose: %w", err)
	}
	return nil
}

func ensureDockerCLI() error {
	if _, err := exec.LookPath("docker"); err == nil {
		logger.Info("docker CLI already installed")
		return nil
	}

	logger.Info("installing docker CLI", zap.String("version", dockerVersion))

	arch := goArchToDockerArch()
	url := fmt.Sprintf(
		"https://download.docker.com/linux/static/stable/%s/docker-%s.tgz",
		arch, dockerVersion,
	)

	cmd := exec.Command("sh", "-c", fmt.Sprintf(
		`curl -fsSL %s | tar -xz -C /usr/local/bin --strip-components=1 docker/docker`,
		url,
	))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker install failed: %w", err)
	}

	logger.Info("docker CLI installed successfully")
	return nil
}

func ensureDockerCompose() error {
	cmd := exec.Command("docker", "compose", "version")
	if err := cmd.Run(); err == nil {
		logger.Info("docker compose already installed")
		return nil
	}

	logger.Info("installing docker compose plugin", zap.String("version", dockerComposeVersion))

	arch := goArchToDockerArch()
	url := fmt.Sprintf(
		"https://github.com/docker/compose/releases/download/%s/docker-compose-linux-%s",
		dockerComposeVersion, arch,
	)

	pluginDir := "/usr/local/lib/docker/cli-plugins"
	pluginPath := pluginDir + "/docker-compose"

	cmd = exec.Command("sh", "-c", fmt.Sprintf(
		`mkdir -p %s && curl -fsSL %s -o %s && chmod +x %s`,
		pluginDir, url, pluginPath, pluginPath,
	))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose install failed: %w", err)
	}

	logger.Info("docker compose plugin installed successfully")
	return nil
}

// goArchToDockerArch maps Go's runtime.GOARCH values to Docker's arch naming convention.
func goArchToDockerArch() string {
	switch runtime.GOARCH {
	case "arm64":
		return "aarch64"
	case "386":
		return "i386"
	default:
		return "x86_64"
	}
}

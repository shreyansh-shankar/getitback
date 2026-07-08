package actions

import (
	"fmt"
	"os"
	"time"

	"github.com/shreyansh-shankar/getitback/internal/runtime"
)

// ImportDockerImage loads a Docker image from an archive file.
type ImportDockerImage struct {
	BaseAction
	ImagePath string
	ImageName string
}

func (a *ImportDockerImage) Name() string { return "import_docker_image" }

func (a *ImportDockerImage) Description() string {
	return fmt.Sprintf("Import Docker image from %s", a.ImagePath)
}

func (a *ImportDockerImage) Execute(ctx *runtime.RestoreContext) error {
	args := []string{"load", "-i", a.ImagePath}
	return ctx.Runtime.Exec.Run("docker", args...)
}

func (a *ImportDockerImage) EstimatedDuration() time.Duration { return 30 * time.Second }

// RestoreDockerVolume restores a Docker volume from an archive.
type RestoreDockerVolume struct {
	BaseAction
	VolumeName string
	Archive    string
}

func (a *RestoreDockerVolume) Name() string { return "restore_docker_volume" }

func (a *RestoreDockerVolume) Description() string {
	return fmt.Sprintf("Restore Docker volume %s from %s", a.VolumeName, a.Archive)
}

func (a *RestoreDockerVolume) Execute(ctx *runtime.RestoreContext) error {
	tmpContainer := fmt.Sprintf("getitback-restore-%s-%d", a.VolumeName, os.Getpid())
	script := fmt.Sprintf(
		"docker run --rm --name %s -v %s:/volume -v %s:/backup alpine tar xzf /backup/%s -C /volume",
		tmpContainer, a.VolumeName, archiveParent(a.Archive), archiveName(a.Archive),
	)
	return ctx.Runtime.Exec.RunBash(script)
}

func (a *RestoreDockerVolume) EstimatedDuration() time.Duration { return 30 * time.Second }

// DockerComposeUp runs docker compose up for a project.
type DockerComposeUp struct {
	BaseAction
	ProjectDir string
	Services   []string
}

func (a *DockerComposeUp) Name() string { return "docker_compose_up" }

func (a *DockerComposeUp) Description() string {
	return fmt.Sprintf("Docker compose up in %s", a.ProjectDir)
}

func (a *DockerComposeUp) Execute(ctx *runtime.RestoreContext) error {
	args := []string{"compose", "-f", a.ProjectDir + "/docker-compose.yml", "up", "-d"}
	if len(a.Services) > 0 {
		args = append(args, a.Services...)
	}
	return ctx.Runtime.Exec.Run("docker", args...)
}

func (a *DockerComposeUp) EstimatedDuration() time.Duration { return 30 * time.Second }

func archiveParent(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i]
		}
	}
	return "/"
}

func archiveName(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[i+1:]
		}
	}
	return path
}

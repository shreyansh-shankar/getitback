package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/shreyansh-shankar/getitback/internal/module"
	"github.com/shreyansh-shankar/getitback/internal/runtime"
	"github.com/shreyansh-shankar/getitback/internal/runtime/actions"
)

func (m *DockerModule) Actions(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) ([]actions.Action, error) {
	rt, _ := opts.Runtime.(*runtime.Runtime)
	tmpDir, err := os.MkdirTemp("", "getitback-docker-restore-*")
	if err != nil {
		return nil, fmt.Errorf("docker: create temp dir: %w", err)
	}
	home := homeDir(rt)

	return []actions.Action{
		&actions.ExtractArchive{
			Source:      snap.Path,
			Destination: tmpDir,
		},
		&restoreDockerAction{
			tmpDir: tmpDir,
			home:   home,
			rt:     rt,
		},
	}, nil
}

type restoreDockerAction struct {
	actions.BaseAction
	tmpDir string
	home   string
	rt     *runtime.Runtime
}

func (a *restoreDockerAction) Name() string { return "docker_full_restore" }

func (a *restoreDockerAction) Description() string {
	return "Full Docker environment restoration"
}

func (a *restoreDockerAction) Execute(ctx *runtime.RestoreContext) error {
	eng := &dockerRestoreEngine{
		tmpDir: a.tmpDir,
		home:   a.home,
		rt:     a.rt,
		ctx:    ctx,
	}
	return eng.execute()
}

type dockerRestoreEngine struct {
	tmpDir   string
	home     string
	rt       *runtime.Runtime
	ctx      *runtime.RestoreContext
	manifest dockerBackupManifest
}

func (e *dockerRestoreEngine) execute() error {
	if err := e.loadManifest(); err != nil {
		return err
	}

	available := e.dockerAvailable()

	if !available {
		e.emit("Phase 1/8: Installing Docker Engine")
		if err := e.installDocker(); err != nil {
			e.emit("Manual: Install Docker manually:")
			e.emit("  curl -fsSL https://get.docker.com | sh")
			e.emit("  sudo usermod -aG docker $USER")
			return fmt.Errorf("docker: install failed: %w", err)
		}
		e.emit("Docker Engine installed successfully")
	} else {
		e.emit("Phase 1/8: Docker Engine already installed, skipping")
	}

	e.emit("Phase 2/8: Restoring Docker configuration")
	if err := e.restoreConfigs(); err != nil {
		e.emit("Warning: config restore partial: %s", err)
	}

	e.emit("Phase 3/8: Restoring Docker contexts")
	if err := e.restoreContexts(); err != nil {
		e.emit("Warning: context restore partial: %s", err)
	}

	e.emit("Phase 4/8: Creating Docker networks")
	if err := e.restoreNetworks(); err != nil {
		e.emit("Warning: some networks could not be created: %s", err)
	}

	e.emit("Phase 5/8: Creating Docker volumes")
	if err := e.restoreVolumes(); err != nil {
		e.emit("Warning: some volumes could not be restored: %s", err)
	}

	e.emit("Phase 6/8: Loading Docker images")
	if err := e.restoreImages(); err != nil {
		e.emit("Warning: some images could not be loaded: %s", err)
	}

	e.emit("Phase 7/8: Restoring Compose projects")
	if err := e.restoreCompose(); err != nil {
		e.emit("Warning: compose restore partial: %s", err)
	}

	e.emit("Starting Compose projects...")
	if err := e.composeUp(); err != nil {
		e.emit("Warning: compose up partial: %s", err)
		for _, p := range e.manifest.Compose {
			e.emit("  Manual: cd %s && docker compose up -d", filepath.Dir(p.File))
		}
	}

	e.emit("Phase 8/8: Validation")
	report := e.validate()
	e.emitReport(report)

	return nil
}

func (e *dockerRestoreEngine) loadManifest() error {
	var manifestPath string
	filepath.Walk(e.tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		if filepath.Base(path) == "manifest.json" {
			manifestPath = path
		}
		return nil
	})
	if manifestPath == "" {
		return fmt.Errorf("no manifest found in snapshot")
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}
	return json.Unmarshal(data, &e.manifest)
}

func (e *dockerRestoreEngine) dockerAvailable() bool {
	return exec.Command("docker", "ps", "-a", "--format", "{{.ID}}").Run() == nil
}

func (e *dockerRestoreEngine) installDocker() error {
	cmds := []string{
		"sudo apt-get update -qq",
		"sudo apt-get install -y -qq curl ca-certificates gnupg",
		"sudo install -m 0755 -d /etc/apt/keyrings",
		"curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg",
		"sudo chmod a+r /etc/apt/keyrings/docker.gpg",
		`echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null`,
		"sudo apt-get update -qq",
		"sudo apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-compose-plugin",
		"sudo systemctl enable docker || true",
		"sudo systemctl start docker || true",
	}
	for _, cmd := range cmds {
		e.emit("  %s", cmd[:min(len(cmd), 80)])
		if err := exec.Command("bash", "-c", cmd).Run(); err != nil {
			return fmt.Errorf("install step failed: %w", err)
		}
	}
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (e *dockerRestoreEngine) restoreConfigs() error {
	configsDir := filepath.Join(e.tmpDir, "configs")
	info, err := os.Stat(configsDir)
	if err != nil || !info.IsDir() {
		return nil
	}
	restarted := false
	return filepath.Walk(configsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(configsDir, path)
		if rel == "" {
			return nil
		}
		dstRoot := e.home
		if strings.HasPrefix(rel, "etc/") {
			dstRoot = "/"
		}
		dst := filepath.Join(dstRoot, rel)
		os.MkdirAll(filepath.Dir(dst), 0755)
		data, _ := os.ReadFile(path)
		if err := os.WriteFile(dst, data, 0644); err != nil {
			return fmt.Errorf("write %s: %w", dst, err)
		}
		e.emit("  Restored config: %s", dst)
		if strings.Contains(rel, "daemon.json") && strings.HasPrefix(dst, "/etc/docker/") && !restarted {
			exec.Command("bash", "-c", "sudo systemctl restart docker || true").Run()
			restarted = true
		}
		return nil
	})
}

func (e *dockerRestoreEngine) restoreContexts() error {
	src := filepath.Join(e.tmpDir, "contexts")
	info, err := os.Stat(src)
	if err != nil || !info.IsDir() {
		return nil
	}
	dst := filepath.Join(e.home, ".docker", "contexts")
	os.RemoveAll(dst)
	os.MkdirAll(filepath.Dir(dst), 0700)
	return exec.Command("cp", "-r", src+"/.", dst).Run()
}

func (e *dockerRestoreEngine) restoreNetworks() error {
	for _, net := range e.manifest.Networks {
		if net.Name == "bridge" || net.Name == "host" || net.Name == "none" {
			continue
		}
		if err := exec.Command("docker", "network", "create", "--driver", net.Driver, net.Name).Run(); err != nil {
			e.emit("  Skipping network %s: %s", net.Name, err)
		} else {
			e.emit("  Created network: %s", net.Name)
		}
	}
	return nil
}

func (e *dockerRestoreEngine) restoreVolumes() error {
	volumesDir := filepath.Join(e.tmpDir, "volumes")
	volInfo, err := os.Stat(volumesDir)
	hasVolumeData := err == nil && volInfo.IsDir()

	for _, vol := range e.manifest.Volumes {
		if err := exec.Command("docker", "volume", "create", vol).Run(); err != nil {
			e.emit("  Warning: could not create volume %s: %s", vol, err)
			continue
		}
		e.emit("  Created volume: %s", vol)

		if hasVolumeData {
			archivePath := filepath.Join(volumesDir, vol+".tar.gz")
			if _, err := os.Stat(archivePath); err == nil {
				script := fmt.Sprintf(
					"docker run --rm -v %s:/volume -v %s:/backup alpine tar xzf /backup/%s.tar.gz -C /volume",
					vol, volumesDir, vol,
				)
				exec.Command("bash", "-c", script).Run()
				e.emit("  Restored data to volume: %s", vol)
			}
		}
	}
	return nil
}

func (e *dockerRestoreEngine) restoreImages() error {
	imagesDir := filepath.Join(e.tmpDir, "images")
	info, err := os.Stat(imagesDir)
	if err != nil || !info.IsDir() {
		for _, img := range e.manifest.Images {
			e.emit("  Manual: docker pull %s:%s", img.Repository, img.Tag)
		}
		return nil
	}

	for _, img := range e.manifest.Images {
		imageFile := fmt.Sprintf("%s_%s.tar", sanitizeName(img.Repository), sanitizeName(img.Tag))
		imagePath := filepath.Join(imagesDir, imageFile)
		if _, err := os.Stat(imagePath); err != nil {
			e.emit("  Image file not found for %s:%s, need manual pull", img.Repository, img.Tag)
			continue
		}
		if err := exec.Command("docker", "load", "-i", imagePath).Run(); err != nil {
			e.emit("  Warning: could not load image %s:%s: %s", img.Repository, img.Tag, err)
			continue
		}
		e.emit("  Loaded image: %s:%s", img.Repository, img.Tag)
	}
	return nil
}

func (e *dockerRestoreEngine) restoreCompose() error {
	composeDir := filepath.Join(e.tmpDir, "compose")
	info, err := os.Stat(composeDir)
	if err != nil || !info.IsDir() {
		return nil
	}
	entries, err := os.ReadDir(composeDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		srcDir := filepath.Join(composeDir, entry.Name())
		projectDir := filepath.Join(e.home, "Projects", entry.Name())
		os.MkdirAll(projectDir, 0755)
		exec.Command("cp", "-r", srcDir+"/.", projectDir+"/").Run()
		e.emit("  Restored compose project: %s", entry.Name())

		for i := range e.manifest.Compose {
			if e.manifest.Compose[i].Name == entry.Name() {
				e.manifest.Compose[i].File = filepath.Join(projectDir, filepath.Base(e.manifest.Compose[i].File))
			}
		}
	}
	return nil
}

func (e *dockerRestoreEngine) composeUp() error {
	for _, p := range e.manifest.Compose {
		if p.File == "" {
			continue
		}
		if _, err := os.Stat(p.File); err != nil {
			continue
		}
		dir := filepath.Dir(p.File)
		e.emit("  Starting compose project: %s", p.Name)
		if err := exec.Command("docker", "compose", "-f", dir, "up", "-d").Run(); err != nil {
			e.emit("  Warning: compose up for %s failed: %s", p.Name, err)
		} else {
			e.emit("  Compose project %s is running", p.Name)
		}
	}
	return nil
}

func (e *dockerRestoreEngine) validate() *dockerValidationReport {
	r := &dockerValidationReport{}

	r.Installed = exec.Command("docker", "--version").Run() == nil
	if r.Installed {
		r.Successes = append(r.Successes, "Docker Engine installed")
	} else {
		r.Errors = append(r.Errors, "Docker is not installed")
		return r
	}

	r.DaemonRunning = exec.Command("docker", "info").Run() == nil
	if r.DaemonRunning {
		r.Successes = append(r.Successes, "Docker daemon is running")
	} else {
		r.Errors = append(r.Errors, "Docker daemon is not running")
	}

	r.ComposeInstalled = exec.Command("docker", "compose", "version").Run() == nil
	if r.ComposeInstalled {
		r.Successes = append(r.Successes, "Docker Compose plugin installed")
	} else {
		r.Errors = append(r.Errors, "Docker Compose plugin not installed")
	}

	for _, img := range e.manifest.Images {
		check := fmt.Sprintf("%s:%s", img.Repository, img.Tag)
		if exec.Command("docker", "image", "inspect", check).Run() != nil {
			r.MissingImages = append(r.MissingImages, check)
		} else {
			r.RestoredImages++
		}
	}

	for _, vol := range e.manifest.Volumes {
		if exec.Command("docker", "volume", "inspect", vol).Run() != nil {
			r.MissingVolumes = append(r.MissingVolumes, vol)
		} else {
			r.RestoredVolumes++
		}
	}

	for _, net := range e.manifest.Networks {
		if net.Name == "bridge" || net.Name == "host" || net.Name == "none" {
			continue
		}
		if exec.Command("docker", "network", "inspect", net.Name).Run() != nil {
			r.MissingNetworks = append(r.MissingNetworks, net.Name)
		} else {
			r.RestoredNetworks++
		}
	}

	return r
}

func (e *dockerRestoreEngine) emitReport(r *dockerValidationReport) {
	e.emit("")
	e.emit("Docker Restoration Report:")
	e.emit("")

	if r.Installed {
		e.emit("  ✓ Docker Engine installed")
	}
	if r.DaemonRunning {
		e.emit("  ✓ Docker daemon running")
	}
	if r.ComposeInstalled {
		e.emit("  ✓ Docker Compose installed")
	}
	if r.RestoredImages > 0 {
		e.emit("  ✓ %d images loaded", r.RestoredImages)
	}
	if len(r.MissingImages) > 0 {
		e.emit("  ⚠ %d images could not be restored", len(r.MissingImages))
	}
	if r.RestoredVolumes > 0 {
		e.emit("  ✓ %d volumes restored", r.RestoredVolumes)
	}
	if r.RestoredNetworks > 0 {
		e.emit("  ✓ %d networks created", r.RestoredNetworks)
	}

	if len(r.MissingImages) > 0 {
		e.emit("")
		e.emit("  Manual steps required:")
		for _, img := range r.MissingImages {
			e.emit("    docker pull %s", img)
		}
	}
	if len(r.MissingVolumes) > 0 {
		for _, vol := range r.MissingVolumes {
			e.emit("    docker volume create %s", vol)
		}
	}
	if len(r.MissingNetworks) > 0 {
		for _, net := range r.MissingNetworks {
			e.emit("    docker network create %s", net)
		}
	}
	for _, err := range r.Errors {
		e.emit("  ✗ %s", err)
	}
	e.emit("")
}

func (e *dockerRestoreEngine) emit(format string, args ...any) {
	e.ctx.Runtime.Progress.DetailLine(format, args...)
}

type dockerValidationReport struct {
	Installed        bool
	DaemonRunning    bool
	ComposeInstalled bool
	RestoredImages   int
	MissingImages    []string
	RestoredVolumes  int
	MissingVolumes   []string
	RestoredNetworks int
	MissingNetworks  []string
	Successes        []string
	Errors           []string
}

func sanitizeName(name string) string {
	r := strings.NewReplacer(
		"/", "_", ":", "_", ".", "_", "-", "_",
	)
	return r.Replace(name)
}

func homeDir(rt *runtime.Runtime) string {
	home := os.Getenv("HOME")
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	if rt != nil && rt.OS.HomeDir != "" {
		home = rt.OS.HomeDir
	}
	return home
}

var _ actions.Provider = (*DockerModule)(nil)

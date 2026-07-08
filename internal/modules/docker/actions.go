package docker

import (
	"archive/tar"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/shreyansh-shankar/getitback/internal/archive"
	"github.com/shreyansh-shankar/getitback/internal/module"
	"github.com/shreyansh-shankar/getitback/internal/runtime"
	"github.com/shreyansh-shankar/getitback/internal/runtime/actions"
)

func (m *DockerModule) Actions(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) ([]actions.Action, error) {
	rt, _ := opts.Runtime.(*runtime.Runtime)
	workDir := opts.WorkDir
	if workDir == "" {
		workDir = "/tmp"
	}
	home := homeDir(rt)

	return []actions.Action{
		&restoreDockerAction{
			archivePath: snap.Path,
			workDir:     workDir,
			home:        home,
			rt:          rt,
		},
	}, nil
}

type restoreDockerAction struct {
	actions.BaseAction
	archivePath string
	workDir     string
	home        string
	rt          *runtime.Runtime
}

func (a *restoreDockerAction) Name() string { return "docker_full_restore" }

func (a *restoreDockerAction) Description() string {
	return "Full Docker environment restoration"
}

func (a *restoreDockerAction) Execute(ctx *runtime.RestoreContext) error {
	eng := &dockerRestoreEngine{
		archivePath: a.archivePath,
		workDir:     a.workDir,
		home:        a.home,
		rt:          a.rt,
		restoreCtx:  ctx,
	}
	return eng.execute()
}

type dockerRestoreEngine struct {
	archivePath string
	workDir     string
	home        string
	rt          *runtime.Runtime
	restoreCtx  *runtime.RestoreContext
	manifest    dockerBackupManifest
}

func (e *dockerRestoreEngine) execute() error {
	workSub, err := os.MkdirTemp(e.workDir, "getitback-docker-*")
	if err != nil {
		return fmt.Errorf("docker: create work dir: %w", err)
	}
	defer os.RemoveAll(workSub)

	// Phase 0: Install Docker Engine if not available
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

	// Count items for progress reporting
	imgCount, volCount := e.countItems()
	e.emit("Phase 2/8: Restoring Docker configuration")
	e.emit("Phase 3/8: Restoring Docker contexts")
	e.emit("Phase 4/8: Creating Docker networks")
	e.emit("Phase 5/8: Creating and restoring Docker volumes")
	e.emit("Phase 6/8: Loading Docker images")
	e.emit("Phase 7/8: Restoring Compose projects")

	if err := e.streamArchive(workSub, imgCount, volCount); err != nil {
		return err
	}

	// Apply configs, contexts, compose (extracted in streaming pass)
	e.restoreConfigs(filepath.Join(workSub, "configs"))
	e.restoreContexts(filepath.Join(workSub, "contexts"))
	e.restoreNetworks()
	e.restoreCompose(filepath.Join(workSub, "compose"))

	e.emit("Starting Compose projects...")
	e.composeUp()

	e.emit("Phase 8/8: Validation")
	report := e.validate()
	e.emitReport(report)

	return nil
}

func (e *dockerRestoreEngine) countItems() (images, volumes int) {
	r, err := archive.OpenReader(e.archivePath)
	if err != nil {
		return 0, 0
	}
	defer r.Close()
	for {
		hdr, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		name := filepath.ToSlash(hdr.Name)
		switch {
		case strings.HasPrefix(name, "images/") && strings.HasSuffix(name, ".tar"):
			images++
		case strings.HasPrefix(name, "volumes/") && strings.HasSuffix(name, ".tar.gz"):
			volumes++
		}
	}
	return
}

func (e *dockerRestoreEngine) streamArchive(workSub string, imgCount, volCount int) error {
	r, err := archive.OpenReader(e.archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer r.Close()

	var imgIdx, volIdx int

	for {
		hdr, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read archive entry: %w", err)
		}

		name := filepath.ToSlash(hdr.Name)
		switch {
		case name == "manifest.json":
			data, readErr := io.ReadAll(r)
			if readErr != nil {
				return fmt.Errorf("read manifest: %w", readErr)
			}
			if jsonErr := json.Unmarshal(data, &e.manifest); jsonErr != nil {
				return fmt.Errorf("parse manifest: %w", jsonErr)
			}

		case strings.HasPrefix(name, "configs/"):
			if err := archive.WriteEntry(hdr, r, workSub); err != nil {
				e.emit("Warning: config extraction failed: %s", err)
			}

		case strings.HasPrefix(name, "contexts/"):
			if err := archive.WriteEntry(hdr, r, workSub); err != nil {
				e.emit("Warning: context extraction failed: %s", err)
			}

		case strings.HasPrefix(name, "compose/"):
			if err := archive.WriteEntry(hdr, r, workSub); err != nil {
				e.emit("Warning: compose extraction failed: %s", err)
			}

		case strings.HasPrefix(name, "images/") && strings.HasSuffix(name, ".tar"):
			imgIdx++
			imgFile, tmpErr := e.writeTempFile(workSub, "docker-image-*.tar", hdr, r)
			if tmpErr != nil {
				e.emit("Warning: could not extract image: %s", tmpErr)
				continue
			}
			tag := e.imageTagFromPath(name)
			if imgCount > 0 {
				e.emit("Loading image %d/%d: %s", imgIdx, imgCount, tag)
			} else {
				e.emit("Loading image %s", tag)
			}
			if loadErr := exec.Command("docker", "load", "-i", imgFile).Run(); loadErr != nil {
				e.emit("Warning: could not load image %s: %s", tag, loadErr)
			}
			os.Remove(imgFile)

		case strings.HasPrefix(name, "volumes/") && strings.HasSuffix(name, ".tar.gz"):
			volIdx++
			volName := e.volumeNameFromPath(name)
			if volName == "" {
				continue
			}
			volFile, tmpErr := e.writeTempFile(workSub, "docker-vol-*.tar.gz", hdr, r)
			if tmpErr != nil {
				e.emit("Warning: could not extract volume data: %s", tmpErr)
				continue
			}
			if volCount > 0 {
				e.emit("Restoring volume %s (%d/%d)", volName, volIdx, volCount)
			} else {
				e.emit("Restoring volume %s", volName)
			}
			if _, volErr := exec.Command("docker", "volume", "create", volName).Output(); volErr != nil {
				e.emit("Warning: could not create volume %s: %s", volName, volErr)
				os.Remove(volFile)
				continue
			}
			script := fmt.Sprintf(
				"docker run --rm -v %s:/volume -v %s:/backup alpine tar xzf /backup/%s -C /volume",
				volName, filepath.Dir(volFile), filepath.Base(volFile),
			)
			if restErr := exec.Command("bash", "-c", script).Run(); restErr != nil {
				e.emit("Warning: could not restore volume %s data: %s", volName, restErr)
			}
			os.Remove(volFile)

		case strings.HasPrefix(name, "volumes/") && strings.HasSuffix(name, ".json"):
			// Volume metadata — not needed for restore, skip

		case strings.HasPrefix(name, "containers/"):
			// Container metadata — skip during restore
		}
	}
	return nil
}

func (e *dockerRestoreEngine) writeTempFile(dir, pattern string, hdr *tar.Header, r io.Reader) (string, error) {
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

func (e *dockerRestoreEngine) imageTagFromPath(path string) string {
	name := filepath.Base(path)
	name = strings.TrimSuffix(name, ".tar")
	name = strings.ReplaceAll(name, "_", "/")
	if idx := strings.LastIndex(name, "/"); idx > 0 && idx < len(name)-1 {
		tag := name[idx+1:]
		repo := name[:idx]
		return repo + ":" + tag
	}
	return name
}

func (e *dockerRestoreEngine) volumeNameFromPath(path string) string {
	name := filepath.Base(path)
	name = strings.TrimSuffix(name, ".tar.gz")
	return name
}

func (e *dockerRestoreEngine) loadManifest() error {
	r, err := archive.OpenReader(e.archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer r.Close()
	for {
		hdr, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read entry: %w", err)
		}
		if filepath.ToSlash(hdr.Name) == "manifest.json" {
			data, readErr := io.ReadAll(r)
			if readErr != nil {
				return fmt.Errorf("read manifest: %w", readErr)
			}
			return json.Unmarshal(data, &e.manifest)
		}
	}
	return fmt.Errorf("manifest.json not found in archive")
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
		`echo "deb [arch=$(dpkg --arch) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo \"$VERSION_CODENAME\") stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null`,
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

func (e *dockerRestoreEngine) restoreConfigs(dir string) {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return
	}
	restarted := false
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
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
			e.emit("Warning: write config %s: %s", dst, err)
			return nil
		}
		e.emit("Restored config: %s", dst)
		if strings.Contains(rel, "daemon.json") && strings.HasPrefix(dst, "/etc/docker/") && !restarted {
			exec.Command("bash", "-c", "sudo systemctl restart docker || true").Run()
			restarted = true
		}
		return nil
	})
}

func (e *dockerRestoreEngine) restoreContexts(dir string) {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return
	}
	dst := filepath.Join(e.home, ".docker", "contexts")
	os.RemoveAll(dst)
	os.MkdirAll(filepath.Dir(dst), 0700)
	exec.Command("cp", "-r", dir+"/.", dst).Run()
}

func (e *dockerRestoreEngine) restoreNetworks() {
	for _, net := range e.manifest.Networks {
		if net.Name == "bridge" || net.Name == "host" || net.Name == "none" {
			continue
		}
		if err := exec.Command("docker", "network", "create", "--driver", net.Driver, net.Name).Run(); err != nil {
			e.emit("Skipping network %s: %s", net.Name, err)
		} else {
			e.emit("Created network: %s", net.Name)
		}
	}
}

func (e *dockerRestoreEngine) restoreCompose(dir string) {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		srcDir := filepath.Join(dir, entry.Name())
		projectDir := filepath.Join(e.home, "Projects", entry.Name())
		os.MkdirAll(projectDir, 0755)
		exec.Command("cp", "-r", srcDir+"/.", projectDir+"/").Run()
		e.emit("Restored compose project: %s", entry.Name())
		for i := range e.manifest.Compose {
			if e.manifest.Compose[i].Name == entry.Name() {
				e.manifest.Compose[i].File = filepath.Join(projectDir, filepath.Base(e.manifest.Compose[i].File))
			}
		}
	}
}

func (e *dockerRestoreEngine) composeUp() {
	for _, p := range e.manifest.Compose {
		if p.File == "" {
			continue
		}
		if _, err := os.Stat(p.File); err != nil {
			continue
		}
		dir := filepath.Dir(p.File)
		e.emit("Starting compose project: %s", p.Name)
		if err := exec.Command("docker", "compose", "-f", dir, "up", "-d").Run(); err != nil {
			e.emit("Warning: compose up for %s failed: %s", p.Name, err)
		} else {
			e.emit("Compose project %s is running", p.Name)
		}
	}
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
	e.restoreCtx.Runtime.Progress.DetailLine(format, args...)
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

package docker

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/shreyansh-shankar/getitback/internal/archive"
	"github.com/shreyansh-shankar/getitback/internal/module"
	"github.com/shreyansh-shankar/getitback/internal/runtime"
	"github.com/shreyansh-shankar/getitback/internal/runtime/restoreutil"
)

type DockerModule struct{}

func NewModule() *DockerModule { return &DockerModule{} }

func (m *DockerModule) Name() string        { return "docker" }
func (m *DockerModule) Description() string { return "Docker Engine and container ecosystem" }

func (m *DockerModule) Detect() (bool, error) {
	_, err := exec.LookPath("docker")
	return err == nil, nil
}

func (m *DockerModule) Inventory(ctx context.Context) (*module.InventoryResult, error) {
	result := &module.InventoryResult{Module: m.Name(), Detected: true}
	meta := make(map[string]any)

	if ver, err := exec.Command("docker", "--version").Output(); err == nil {
		result.Version = strings.TrimSpace(string(ver))
	}

	installMethod := detectInstallMethod()
	if installMethod != "" {
		meta["installMethod"] = installMethod
	}

	rootDir := getDockerRootDir()
	if rootDir != "" {
		meta["rootDir"] = rootDir
	}

	if isRootless() {
		meta["rootless"] = true
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	wg.Add(1)
	go func() {
		defer wg.Done()
		containers, running, stopped, restartWarn := collectContainers()
		mu.Lock()
		meta["containers"] = containers
		meta["runningContainers"] = running
		meta["stoppedContainers"] = stopped
		if restartWarn {
			result.Warnings = append(result.Warnings, "Some containers are running without a restart policy")
		}
		mu.Unlock()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		images, imageStorage, dangling := collectImages()
		mu.Lock()
		if images > 0 {
			meta["images"] = images
			if imageStorage > 0 {
				meta["imageStorage"] = imageStorage
			}
		}
		if dangling > 0 {
			meta["danglingImages"] = dangling
			result.Warnings = append(result.Warnings, fmt.Sprintf("%d dangling images detected", dangling))
		}
		mu.Unlock()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		volumes, volStorage := collectVolumes()
		mu.Lock()
		if volumes > 0 {
			meta["volumes"] = volumes
			if volStorage > 0 {
				meta["volumeStorage"] = volStorage
			}
		}
		mu.Unlock()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		networks, customNets := collectNetworks()
		mu.Lock()
		if networks > 0 {
			meta["networks"] = networks
			meta["customNetworks"] = customNets
		}
		mu.Unlock()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		projects, projectNames := discoverCompose()
		mu.Lock()
		if len(projectNames) > 0 {
			meta["composeProjects"] = projects
			meta["composeProjectNames"] = projectNames
		}
		mu.Unlock()
	}()

	wg.Wait()

	if len(meta) > 0 {
		result.Metadata = meta
	}

	return result, nil
}

func detectInstallMethod() string {
	if _, err := os.Stat("/usr/bin/docker"); err == nil {
		return "system"
	}
	if _, err := os.Stat("/usr/local/bin/docker"); err == nil {
		return "manual"
	}
	if _, err := os.Stat("/snap/bin/docker"); err == nil {
		return "snap"
	}
	out, err := exec.Command("docker", "info", "--format", "{{.OperatingMode}}").Output()
	if err == nil && strings.Contains(strings.ToLower(string(out)), "desktop") {
		return "docker-desktop"
	}
	return "unknown"
}

func getDockerRootDir() string {
	out, err := exec.Command("docker", "info", "--format", "{{.DockerRootDir}}").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func isRootless() bool {
	out, err := exec.Command("docker", "info", "--format", "{{.SecurityOptions}}").Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "rootless")
}

func collectContainers() (total, running, stopped int, restartWarn bool) {
	out, err := exec.Command("docker", "ps", "-a", "--format", "{{.ID}}\t{{.Names}}\t{{.Status}}").Output()
	if err != nil {
		return 0, 0, 0, false
	}
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	var containerNames []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 {
			total++
			continue
		}
		total++
		status := strings.ToLower(parts[2])
		if strings.HasPrefix(status, "up") {
			running++
			containerNames = append(containerNames, parts[1])
		} else {
			stopped++
		}
	}

	if running > 0 && len(containerNames) > 0 {
		checkRestartPolicies(containerNames, &restartWarn)
	}

	return
}

func checkRestartPolicies(names []string, restartWarn *bool) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	sem := make(chan struct{}, 5)

	for _, name := range names {
		wg.Add(1)
		go func(n string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			policy, err := exec.Command("docker", "inspect", "--format", "{{.HostConfig.RestartPolicy.Name}}", n).Output()
			if err != nil {
				return
			}
			mu.Lock()
			if strings.TrimSpace(string(policy)) == "no" {
				*restartWarn = true
			}
			mu.Unlock()
		}(name)
	}
	wg.Wait()
}

func collectImages() (count int, storage int64, dangling int) {
	out, err := exec.Command("docker", "images", "--format", "{{.Repository}}:{{.Tag}}\t{{.Size}}").Output()
	if err != nil {
		return 0, 0, 0
	}
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		count++
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) == 2 {
			storage += parseDockerSize(parts[1])
		}
	}

	out, err = exec.Command("docker", "images", "-f", "dangling=true", "-q").Output()
	if err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if line != "" {
				dangling++
			}
		}
	}

	return
}

func collectVolumes() (count int, storage int64) {
	out, err := exec.Command("docker", "volume", "ls", "-q").Output()
	if err != nil {
		return 0, 0
	}
	volumes := strings.Fields(string(out))
	count = len(volumes)

	if count > 0 {
		var wg sync.WaitGroup
		var mu sync.Mutex
		sem := make(chan struct{}, 3)

		for _, vol := range volumes {
			wg.Add(1)
			go func(v string) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				mountPoint, err := exec.Command("docker", "volume", "inspect", "--format", "{{.Mountpoint}}", v).Output()
				if err != nil {
					return
				}
				mp := strings.TrimSpace(string(mountPoint))
				if mp == "" {
					return
				}
				var size int64
				filepath.Walk(mp, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return filepath.SkipDir
					}
					if !info.IsDir() {
						size += info.Size()
					}
					return nil
				})
				mu.Lock()
				storage += size
				mu.Unlock()
			}(vol)
		}
		wg.Wait()
	}

	return
}

func collectNetworks() (total, custom int) {
	out, err := exec.Command("docker", "network", "ls", "--format", "{{.Name}}\t{{.Driver}}").Output()
	if err != nil {
		return 0, 0
	}
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		total++
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) == 2 && parts[0] != "bridge" && parts[0] != "host" && parts[0] != "none" {
			custom++
		}
	}
	return
}

func discoverCompose() (count int, names []string) {
	home, _ := os.UserHomeDir()
	searchDirs := []string{
		filepath.Join(home, "Projects"),
		filepath.Join(home, "Code"),
		filepath.Join(home, "Workspace"),
		filepath.Join(home, "Development"),
		home,
	}

	for _, dir := range searchDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			composeFile := filepath.Join(dir, entry.Name(), "docker-compose.yml")
			if _, err := os.Stat(composeFile); os.IsNotExist(err) {
				composeFile = filepath.Join(dir, entry.Name(), "docker-compose.yaml")
			}
			if _, err := os.Stat(composeFile); os.IsNotExist(err) {
				composeFile = filepath.Join(dir, entry.Name(), "compose.yaml")
			}
			if _, err := os.Stat(composeFile); err == nil {
				count++
				names = append(names, entry.Name())
			}
			if count >= 50 {
				return
			}
		}
		if dir == home {
			break
		}
	}
	return
}

func parseDockerSize(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	var val float64
	var unit string
	fmt.Sscanf(s, "%f%s", &val, &unit)
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "b":
		return int64(val)
	case "kb", "kib":
		return int64(val * 1024)
	case "mb", "mib":
		return int64(val * 1024 * 1024)
	case "gb", "gib":
		return int64(val * 1024 * 1024 * 1024)
	case "tb", "tib":
		return int64(val * 1024 * 1024 * 1024 * 1024)
	default:
		return int64(val)
	}
}

type dockerBackupManifest struct {
	Containers []containerInfo  `json:"containers,omitempty"`
	Volumes    []string         `json:"volumes,omitempty"`
	Networks   []networkInfo    `json:"networks,omitempty"`
	Images     []imageInfo      `json:"images,omitempty"`
	Compose    []composeProject `json:"compose,omitempty"`
	Configs    []string         `json:"configs,omitempty"`
	Contexts   []string         `json:"contexts,omitempty"`
}

type containerInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Image  string `json:"image"`
	Status string `json:"status"`
}

type networkInfo struct {
	Name   string `json:"name"`
	Driver string `json:"driver"`
	Scope  string `json:"scope"`
}

type imageInfo struct {
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
	Size       string `json:"size"`
}

type composeProject struct {
	Name string `json:"name"`
	File string `json:"file"`
}

// --- Enhanced restore interfaces ---

func (m *DockerModule) Dependencies(ctx context.Context) []module.Dependency {
	return []module.Dependency{
		{Type: module.DepSystemPkg, Package: "docker.io", Hint: "Docker Engine"},
		{Type: module.DepCommand, Command: "docker-compose", Hint: "Docker Compose", Optional: true},
	}
}

func (m *DockerModule) Install(ctx context.Context, opts module.RestoreOptions) error {
	rt, _ := opts.Runtime.(*runtime.Runtime)
	if rt != nil {
		return rt.Pkg.Install("docker.io")
	}
	return exec.Command("sudo", "apt-get", "install", "-y", "-qq", "docker.io").Run()
}

func (m *DockerModule) Configure(ctx context.Context, opts module.RestoreOptions) error {
	return nil
}

func (m *DockerModule) Validate(ctx context.Context, snap module.Snapshot) (*module.ValidateResult, error) {
	v := restoreutil.NewValidation("docker")

	if ver, err := restoreutil.CheckExecOutput("docker", "--version"); err == nil {
		v.Version(strings.TrimSpace(ver))
	}
	v.Check(restoreutil.CommandExists("docker"), "Docker binary installed")

	out, err := exec.Command("docker", "info").Output()
	v.Check(err == nil && len(out) > 0, "Docker daemon running")

	if restoreutil.CommandExists("docker-compose") {
		v.Recovered("Docker Compose available")
	}

	tmpDir, err := os.MkdirTemp("", "getitback-docker-validate-*")
	if err == nil {
		defer os.RemoveAll(tmpDir)
		if err := archive.Extract(snap.Path, tmpDir); err == nil {
			manifestPath := filepath.Join(tmpDir, "manifest.json")
			if data, err := os.ReadFile(manifestPath); err == nil {
				var manifest dockerBackupManifest
				if json.Unmarshal(data, &manifest) == nil {
					if len(manifest.Containers) > 0 {
						v.Recovered(fmt.Sprintf("container configs (%d)", len(manifest.Containers)))
					}
					if len(manifest.Images) > 0 {
						count := countRestoredImages(manifest)
						v.Recovered(fmt.Sprintf("images restored: %d/%d", count, len(manifest.Images)))
						if count < len(manifest.Images) {
							v.Warn("not all images could be verified")
						}
					}
					if len(manifest.Volumes) > 0 {
						v.Recovered(fmt.Sprintf("volume configs (%d)", len(manifest.Volumes)))
					}
					if len(manifest.Networks) > 0 {
						v.Recovered(fmt.Sprintf("network configs (%d)", len(manifest.Networks)))
					}
					if len(manifest.Compose) > 0 {
						v.Recovered(fmt.Sprintf("compose projects (%d)", len(manifest.Compose)))
					}
				}
			}
		}
	}

	v.Confidence(85)
	return v.Result(), nil
}

func countRestoredImages(manifest dockerBackupManifest) int {
	count := 0
	for _, img := range manifest.Images {
		if img.Repository == "" || img.Tag == "" {
			continue
		}
		out, _ := exec.Command("docker", "image", "inspect", img.Repository+":"+img.Tag).Output()
		if len(out) > 0 {
			count++
		}
	}
	return count
}

func (m *DockerModule) Backup(ctx context.Context, opts module.BackupOptions) (*module.BackupResult, error) {
	var manifest dockerBackupManifest
	var entries []archive.Entry
	var tmpFiles []string

	defer func() {
		for _, f := range tmpFiles {
			os.Remove(f)
		}
	}()

	home, _ := os.UserHomeDir()

	dockerAvailable := func() bool {
		_, err := exec.Command("docker", "ps", "-a", "--format", "{{.ID}}").Output()
		return err == nil
	}

	if dockerAvailable() {
		out, err := exec.Command("docker", "ps", "-a", "--format", "{{.ID}}\t{{.Names}}\t{{.Image}}\t{{.Status}}").Output()
		if err == nil {
			scanner := bufio.NewScanner(strings.NewReader(string(out)))
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" {
					continue
				}
				parts := strings.SplitN(line, "\t", 4)
				if len(parts) < 4 {
					continue
				}
				manifest.Containers = append(manifest.Containers, containerInfo{
					ID: parts[0], Name: parts[1], Image: parts[2], Status: parts[3],
				})
				cfg, err := exec.Command("docker", "inspect", parts[1]).Output()
				if err != nil {
					continue
				}
				tmpFile := filepath.Join(os.TempDir(), "getitback-docker-container-"+parts[1]+".json")
				os.WriteFile(tmpFile, cfg, 0600)
				tmpFiles = append(tmpFiles, tmpFile)
				entries = append(entries, archive.Entry{
					Source: tmpFile, ArchivePath: "containers/" + parts[1] + ".json",
				})
			}
		}

		out, err = exec.Command("docker", "images", "--format", "{{.Repository}}:{{.Tag}}\t{{.Size}}").Output()
		if err == nil {
			scanner := bufio.NewScanner(strings.NewReader(string(out)))
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" {
					continue
				}
				parts := strings.SplitN(line, "\t", 2)
				size := ""
				if len(parts) == 2 {
					size = parts[1]
				}
				repo, tag, _ := strings.Cut(parts[0], ":")
				manifest.Images = append(manifest.Images, imageInfo{
					Repository: repo, Tag: tag, Size: size,
				})
			}
		}

		for _, img := range manifest.Images {
			if img.Repository == "" || img.Tag == "" {
				continue
			}
			imageRef := img.Repository + ":" + img.Tag
			if strings.Contains(imageRef, "<none>") {
				continue
			}
			imageFile := filepath.Join(os.TempDir(), "getitback-docker-image-"+sanitizeFileName(img.Repository)+"_"+sanitizeFileName(img.Tag)+".tar")
			if err := exec.Command("docker", "save", "-o", imageFile, imageRef).Run(); err != nil {
				continue
			}
			tmpFiles = append(tmpFiles, imageFile)
			entries = append(entries, archive.Entry{
				Source: imageFile, ArchivePath: "images/" + sanitizeFileName(img.Repository) + "_" + sanitizeFileName(img.Tag) + ".tar",
			})
		}

		out, err = exec.Command("docker", "volume", "ls", "-q").Output()
		if err == nil {
			for _, vol := range strings.Fields(string(out)) {
				manifest.Volumes = append(manifest.Volumes, vol)
				info, err := exec.Command("docker", "volume", "inspect", vol).Output()
				if err != nil {
					continue
				}
				tmpFile := filepath.Join(os.TempDir(), "getitback-docker-volume-"+vol+".json")
				os.WriteFile(tmpFile, info, 0600)
				tmpFiles = append(tmpFiles, tmpFile)
				entries = append(entries, archive.Entry{
					Source: tmpFile, ArchivePath: "volumes/" + vol + ".json",
				})

				tarFile := filepath.Join(os.TempDir(), "getitback-docker-volume-"+vol+".tar.gz")
				if exec.Command("docker", "run", "--rm",
					"-v", vol+":/volume",
					"-v", os.TempDir()+":/backup",
					"alpine:latest",
					"tar", "czf", "/backup/getitback-docker-volume-"+vol+".tar.gz",
					"-C", "/volume", ".",
				).Run() == nil {
					tmpFiles = append(tmpFiles, tarFile)
					entries = append(entries, archive.Entry{
						Source: tarFile, ArchivePath: "volumes/" + vol + ".tar.gz",
					})
				}
			}
		}

		out, err = exec.Command("docker", "network", "ls", "--format", "{{.Name}}\t{{.Driver}}\t{{.Scope}}").Output()
		if err == nil {
			scanner := bufio.NewScanner(strings.NewReader(string(out)))
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" {
					continue
				}
				parts := strings.SplitN(line, "\t", 3)
				if len(parts) < 3 {
					continue
				}
				manifest.Networks = append(manifest.Networks, networkInfo{
					Name: parts[0], Driver: parts[1], Scope: parts[2],
				})
			}
		}
	}

	searchDirs := []string{
		filepath.Join(home, "Projects"),
		filepath.Join(home, "Code"),
		filepath.Join(home, "Workspace"),
		filepath.Join(home, "Development"),
	}
	for _, dir := range searchDirs {
		entries2, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries2 {
			if !entry.IsDir() {
				continue
			}
			for _, name := range []string{"docker-compose.yml", "docker-compose.yaml", "compose.yaml"} {
				p := filepath.Join(dir, entry.Name(), name)
				if _, err := os.Stat(p); err == nil {
					manifest.Compose = append(manifest.Compose, composeProject{
						Name: entry.Name(), File: p,
					})
					entries = append(entries, archive.Entry{
						Source: p, ArchivePath: "compose/" + entry.Name() + "/" + name,
					})
				}
			}
		}
	}

	configPaths := []string{
		"/etc/docker/daemon.json",
		filepath.Join(home, ".docker", "config.json"),
		filepath.Join(home, ".docker", "daemon.json"),
	}
	for _, p := range configPaths {
		if _, err := os.Stat(p); err == nil {
			manifest.Configs = append(manifest.Configs, p)
			rel := strings.TrimPrefix(p, home)
			rel = strings.TrimPrefix(rel, "/")
			entries = append(entries, archive.Entry{
				Source: p, ArchivePath: "configs/" + rel,
			})
		}
	}

	contextDir := filepath.Join(home, ".docker", "contexts")
	if info, err := os.Stat(contextDir); err == nil && info.IsDir() {
		manifest.Contexts = append(manifest.Contexts, contextDir)
		entries = append(entries, archive.Entry{
			Source: contextDir, ArchivePath: "contexts",
		})
	}

	if len(entries) == 0 {
		return nil, nil
	}

	tmpMeta, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("docker: marshal manifest: %w", err)
	}
	metaFile := filepath.Join(os.TempDir(), "getitback-docker-manifest.json")
	if err := os.WriteFile(metaFile, tmpMeta, 0600); err != nil {
		return nil, fmt.Errorf("docker: write manifest: %w", err)
	}
	tmpFiles = append(tmpFiles, metaFile)
	entries = append(entries, archive.Entry{
		Source: metaFile, ArchivePath: "manifest.json",
	})

	snapshot, err := archive.CreateSnapshot(opts.SnapshotsDir, m.Name(), entries)
	if err != nil {
		return nil, err
	}
	if snapshot == nil {
		return nil, nil
	}
	contents := []string{}
	if len(manifest.Containers) > 0 {
		contents = append(contents, fmt.Sprintf("containers (%d)", len(manifest.Containers)))
	}
	if len(manifest.Images) > 0 {
		contents = append(contents, fmt.Sprintf("images (%d)", len(manifest.Images)))
	}
	if len(manifest.Volumes) > 0 {
		contents = append(contents, fmt.Sprintf("volumes (%d)", len(manifest.Volumes)))
	}
	if len(manifest.Networks) > 0 {
		contents = append(contents, fmt.Sprintf("networks (%d)", len(manifest.Networks)))
	}
	if len(manifest.Compose) > 0 {
		contents = append(contents, fmt.Sprintf("compose projects (%d)", len(manifest.Compose)))
	}
	if len(manifest.Configs) > 0 {
		contents = append(contents, fmt.Sprintf("config files (%d)", len(manifest.Configs)))
	}
	if len(manifest.Contexts) > 0 {
		contents = append(contents, "Docker contexts")
	}
	return &module.BackupResult{
		Module:    m.Name(),
		Snapshots: []module.Snapshot{{
			Module: m.Name(), Path: snapshot.Path, Size: snapshot.Size, Checksum: snapshot.Checksum,
			OriginalSize: snapshot.OriginalSize, FileCount: snapshot.FileCount,
		}},
		Contents: contents,
	}, nil
}

func (m *DockerModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	rt, _ := opts.Runtime.(*runtime.Runtime)

	tmpDir, err := os.MkdirTemp(opts.WorkDir, "getitback-restore-docker-*")
	if err != nil {
		return fmt.Errorf("docker: create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if rt != nil {
		if err := rt.Archive.Extract(snap.Path, tmpDir); err != nil {
			return fmt.Errorf("docker: extract snapshot: %w", err)
		}
	} else {
		if err := archive.Extract(snap.Path, tmpDir); err != nil {
			return fmt.Errorf("docker: extract snapshot: %w", err)
		}
	}

	var manifestPath string
	filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		if filepath.Base(path) == "manifest.json" {
			manifestPath = path
		}
		return nil
	})

	if manifestPath == "" {
		return fmt.Errorf("docker: no manifest found in snapshot")
	}

	var data []byte
	if rt != nil {
		data, err = rt.FS.ReadFile(manifestPath)
	} else {
		data, err = os.ReadFile(manifestPath)
	}
	if err != nil {
		return fmt.Errorf("docker: read manifest: %w", err)
	}

	var manifest dockerBackupManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("docker: parse manifest: %w", err)
	}

	home := os.Getenv("HOME")
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	if rt != nil && rt.OS.HomeDir != "" {
		home = rt.OS.HomeDir
	}

	{
		configsDir := filepath.Join(tmpDir, "configs")
		if info, err := os.Stat(configsDir); err == nil && info.IsDir() {
			filepath.Walk(configsDir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				rel, _ := filepath.Rel(configsDir, path)
				if rel == "" {
					return nil
				}
				dstRoot := home
				if strings.HasPrefix(rel, "etc/") {
					dstRoot = "/"
				}
				dst := filepath.Join(dstRoot, rel)
				if rt != nil {
					rt.FS.MkdirAll(filepath.Dir(dst), 0755)
				} else {
					os.MkdirAll(filepath.Dir(dst), 0755)
				}
				fileData, _ := os.ReadFile(path)
				os.WriteFile(dst, fileData, 0644)
				return nil
			})
		}
	}

	{
		contextsDir := filepath.Join(tmpDir, "contexts")
		if info, err := os.Stat(contextsDir); err == nil && info.IsDir() {
			dst := filepath.Join(home, ".docker", "contexts")
			os.RemoveAll(dst)
			if rt != nil {
				rt.FS.MkdirAll(filepath.Dir(dst), 0700)
				rt.FS.CopyDir(contextsDir, dst)
			} else {
				os.MkdirAll(filepath.Dir(dst), 0700)
				exec.Command("cp", "-r", contextsDir, dst).Run()
			}
		}
	}

	return nil
}

func (m *DockerModule) Verify(ctx context.Context, snap module.Snapshot) (*module.VerifyResult, error) {
	info, err := os.Stat(snap.Path)
	if err != nil {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{err.Error()}}, nil
	}
	if info.Size() == 0 {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{"empty snapshot"}}, nil
	}
	return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: true}, nil
}

func (m *DockerModule) Doctor(ctx context.Context) (*module.DoctorResult, error) {
	result := &module.DoctorResult{Module: m.Name(), Status: module.DoctorStatusOK}

	out, err := exec.Command("docker", "system", "df", "--format", "{{.Type}}\t{{.Size}}").Output()
	if err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(out)))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(strings.ToLower(line), "build cache") {
				parts := strings.SplitN(line, "\t", 2)
				if len(parts) == 2 {
					size := strings.TrimSpace(parts[1])
					if size != "0B" {
						result.Issues = append(result.Issues, module.DoctorIssue{
							Severity: "info",
							Message:  fmt.Sprintf("Docker build cache is using %s", size),
							Help:     "Run: docker builder prune",
						})
					}
				}
			}
		}
	}

	if _, err := exec.Command("docker", "volume", "ls", "-qf", "dangling=true").Output(); err == nil {
		out, _ := exec.Command("docker", "volume", "ls", "-qf", "dangling=true").Output()
		if len(strings.TrimSpace(string(out))) > 0 {
			result.Issues = append(result.Issues, module.DoctorIssue{
				Severity: "warning",
				Message:  "Orphaned Docker volumes detected",
				Help:     "Run: docker volume prune",
			})
		}
	}

	if len(result.Issues) > 0 {
		result.Status = module.DoctorStatusWarning
	}
	return result, nil
}

func sanitizeFileName(name string) string {
	r := strings.NewReplacer(
		"/", "_", ":", "_", ".", "_", "-", "_", " ", "_",
	)
	return r.Replace(name)
}

package docker

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/shreyansh-shankar/getitback/internal/archive"
	"github.com/shreyansh-shankar/getitback/internal/module"
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
			result.Warnings = append(result.Warnings, fmt.Sprintf("%d dangling images detected — consider cleaning them up", dangling))
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

func (m *DockerModule) Backup(ctx context.Context, opts module.BackupOptions) (*module.BackupResult, error) {
	var entries []archive.Entry

	composeFiles := discoverComposeFiles()
	for _, f := range composeFiles {
		entries = append(entries, archive.Entry{
			Source: f, ArchivePath: "compose/" + filepath.Base(filepath.Dir(f)) + "/" + filepath.Base(f),
		})
	}

	dockerfile := filepath.Join(".", "Dockerfile")
	if _, err := os.Stat(dockerfile); err == nil {
		entries = append(entries, archive.Entry{
			Source: dockerfile, ArchivePath: "Dockerfile",
		})
	}

	if len(entries) == 0 {
		return nil, nil
	}

	snapshot, err := archive.CreateSnapshot(opts.SnapshotsDir, m.Name(), entries)
	if err != nil {
		return nil, err
	}
	if snapshot == nil {
		return nil, nil
	}
	return &module.BackupResult{
		Module: m.Name(),
		Snapshots: []module.Snapshot{{
			Module: m.Name(), Path: snapshot.Path, Size: snapshot.Size, Checksum: snapshot.Checksum,
		}},
	}, nil
}

func discoverComposeFiles() []string {
	home, _ := os.UserHomeDir()
	searchDirs := []string{
		filepath.Join(home, "Projects"),
		filepath.Join(home, "Code"),
		filepath.Join(home, "Workspace"),
		filepath.Join(home, "Development"),
	}
	var files []string
	for _, dir := range searchDirs {
		entries, _ := os.ReadDir(dir)
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			for _, name := range []string{"docker-compose.yml", "docker-compose.yaml", "compose.yaml"} {
				p := filepath.Join(dir, entry.Name(), name)
				if _, err := os.Stat(p); err == nil {
					files = append(files, p)
				}
			}
		}
	}
	return files
}

func (m *DockerModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
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

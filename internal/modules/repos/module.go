package repos

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/shreyansh-shankar/getitback/internal/archive"
	"github.com/shreyansh-shankar/getitback/internal/module"
	"github.com/shreyansh-shankar/getitback/internal/runtime"
	"github.com/shreyansh-shankar/getitback/internal/runtime/actions"
	"github.com/shreyansh-shankar/getitback/internal/runtime/restoreutil"
)

type ReposModule struct{}

func NewModule() *ReposModule { return &ReposModule{} }

func (m *ReposModule) Name() string        { return "repos" }
func (m *ReposModule) Description() string { return "Git repository discovery" }

func (m *ReposModule) Detect() (bool, error) {
	return restoreutil.CommandExists("git"), nil
}

type repoInfo struct {
	name       string
	path       string
	remote     string
	provider   string
	branch     string
	defaultBr  string
	dirty      bool
	ahead      int
	behind     int
	lastCommit time.Time
	hasSubmod  bool
	lfsEnabled bool
}

var devDirs = []string{
	"Projects",
	"Code",
	"Workspace",
	"Development",
}

func (m *ReposModule) Inventory(ctx context.Context) (*module.InventoryResult, error) {
	result := &module.InventoryResult{Module: m.Name(), Detected: true}
	meta := make(map[string]any)

	home, _ := os.UserHomeDir()
	var repos []repoInfo

	for _, dir := range devDirs {
		searchPath := filepath.Join(home, dir)
		entries, err := os.ReadDir(searchPath)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			repoPath := filepath.Join(searchPath, entry.Name())
			gitDir := filepath.Join(repoPath, ".git")
			if info, err := os.Stat(gitDir); err != nil || !info.IsDir() {
				continue
			}
			repos = append(repos, repoInfo{name: entry.Name(), path: repoPath})
		}
	}

	gitDir := filepath.Join(home, ".git")
	if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
		isBare := false
		if cfg, err := os.ReadFile(filepath.Join(gitDir, "config")); err == nil {
			if strings.Contains(string(cfg), "bare = true") {
				isBare = true
			}
		}
		if isBare {
			repos = append(repos, repoInfo{name: ".dotfiles (bare)", path: home + "/.git"})
		}
	}

	if len(repos) == 0 {
		return result, nil
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, 10)
	var mu sync.Mutex

	for i := range repos {
		wg.Add(1)
		go func(r *repoInfo) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			fetchRepoInfo(r)
			mu.Lock()
			mu.Unlock()
		}(&repos[i])
	}
	wg.Wait()

	total := len(repos)
	var dirty, noRemote, github, gitlab, localOnly int
	var providers []string

	for _, r := range repos {
		if r.dirty {
			dirty++
		}
		if r.remote == "" {
			noRemote++
		}
		switch r.provider {
		case "github.com":
			github++
		case "gitlab.com":
			gitlab++
		case "":
			localOnly++
		}
		if r.provider != "" {
			providers = append(providers, r.provider)
		}
	}

	meta["totalRepos"] = total
	meta["dirtyRepos"] = dirty
	meta["noRemoteRepos"] = noRemote
	meta["githubRepos"] = github
	meta["gitlabRepos"] = gitlab
	meta["localOnlyRepos"] = localOnly

	if len(providers) > 0 {
		providerSet := make(map[string]int)
		for _, p := range providers {
			providerSet[p]++
		}
		var providerSummary []string
		for p, c := range providerSet {
			providerSummary = append(providerSummary, fmt.Sprintf("%s:%d", p, c))
		}
		meta["remoteProviders"] = providerSummary
	}

	if dirty > 0 {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("%d repositories have uncommitted changes", dirty))
	}
	if noRemote > 0 {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("%d repositories have no remote configured", noRemote))
	}

	if len(meta) > 0 {
		result.Metadata = meta
	}
	return result, nil
}

func fetchRepoInfo(r *repoInfo) {
	git := func(args ...string) (string, error) {
		cmd := exec.Command("git", append([]string{
			"-C", r.path,
		}, args...)...)
		out, err := cmd.Output()
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(out)), nil
	}

	if branch, err := git("rev-parse", "--abbrev-ref", "HEAD"); err == nil {
		r.branch = branch
	}

	if def, err := git("symbolic-ref", "refs/remotes/origin/HEAD"); err == nil {
		r.defaultBr = strings.TrimPrefix(def, "refs/remotes/origin/")
	}

	if remote, err := git("config", "--get", "remote.origin.url"); err == nil {
		r.remote = remote
		for _, provider := range []string{"github.com", "gitlab.com", "bitbucket.org"} {
			if strings.Contains(remote, provider) {
				r.provider = provider
				break
			}
		}
	}

	if out, err := git("status", "--porcelain"); err == nil {
		r.dirty = strings.TrimSpace(out) != ""
	}

	if out, err := git("rev-list", "--count", "--left-right", "HEAD...@{u}"); err == nil {
		parts := strings.Fields(out)
		if len(parts) == 2 {
			fmt.Sscanf(parts[0], "%d", &r.ahead)
			fmt.Sscanf(parts[1], "%d", &r.behind)
		}
	}

	if out, err := git("log", "-1", "--format=%ct"); err == nil {
		var ts int64
		fmt.Sscanf(out, "%d", &ts)
		r.lastCommit = time.Unix(ts, 0)
	}

	if _, err := git("submodule", "status"); err == nil {
		r.hasSubmod = true
	}

	if _, err := git("lfs", "env"); err == nil {
		r.lfsEnabled = true
	}
}

type repoBackupEntry struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Remote     string `json:"remote,omitempty"`
	Provider   string `json:"provider,omitempty"`
	Branch     string `json:"branch"`
	DefaultBr  string `json:"defaultBranch,omitempty"`
	Dirty      bool   `json:"dirty"`
	Ahead      int    `json:"ahead"`
	Behind     int    `json:"behind"`
	HasSubmod  bool   `json:"hasSubmodules"`
	LFSEnabled bool   `json:"lfsEnabled"`
	HEADHash   string `json:"headHash"`
}

type reposBackupManifest struct {
	Repos []repoBackupEntry `json:"repositories"`
	Total int               `json:"total"`
	Dirty int               `json:"dirty"`
}

func (m *ReposModule) Backup(ctx context.Context, opts module.BackupOptions) (*module.BackupResult, error) {
	home, _ := os.UserHomeDir()
	var repos []repoInfo

	for _, dir := range devDirs {
		searchPath := filepath.Join(home, dir)
		entries, err := os.ReadDir(searchPath)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			repoPath := filepath.Join(searchPath, entry.Name())
			gitDir := filepath.Join(repoPath, ".git")
			if info, err := os.Stat(gitDir); err != nil || !info.IsDir() {
				continue
			}
			repos = append(repos, repoInfo{name: entry.Name(), path: repoPath})
		}
	}

	if len(repos) == 0 {
		return nil, nil
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, 10)
	for i := range repos {
		wg.Add(1)
		go func(r *repoInfo) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			fetchRepoInfo(r)
		}(&repos[i])
	}
	wg.Wait()

	var manifest reposBackupManifest
	for _, r := range repos {
		headHash := ""
		if hash, err := exec.Command("git", "-C", r.path, "rev-parse", "HEAD").Output(); err == nil {
			headHash = strings.TrimSpace(string(hash))
		}

		manifest.Repos = append(manifest.Repos, repoBackupEntry{
			Name:       r.name,
			Path:       r.path,
			Remote:     r.remote,
			Provider:   r.provider,
			Branch:     r.branch,
			DefaultBr:  r.defaultBr,
			Dirty:      r.dirty,
			Ahead:      r.ahead,
			Behind:     r.behind,
			HasSubmod:  r.hasSubmod,
			LFSEnabled: r.lfsEnabled,
			HEADHash:   headHash,
		})
		if r.dirty {
			manifest.Dirty++
		}
	}
	manifest.Total = len(repos)

	tmpMeta, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("repos: marshal manifest: %w", err)
	}
	metaFile := filepath.Join(os.TempDir(), "getitback-repos-manifest.json")
	if err := os.WriteFile(metaFile, tmpMeta, 0600); err != nil {
		return nil, fmt.Errorf("repos: write manifest: %w", err)
	}
	defer os.Remove(metaFile)

	entries := []archive.Entry{
		{Source: metaFile, ArchivePath: "manifest.json"},
	}

	snapshot, err := archive.CreateSnapshot(opts.SnapshotsDir, m.Name(), entries)
	if err != nil {
		return nil, err
	}
	if snapshot == nil {
		return nil, nil
	}
	contents := []string{fmt.Sprintf("repository manifest (%d repos)", manifest.Total)}
	if manifest.Dirty > 0 {
		contents = append(contents, fmt.Sprintf("uncommitted changes (%d)", manifest.Dirty))
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

func (m *ReposModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	tmpDir, err := os.MkdirTemp("", "getitback-restore-repos-*")
	if err != nil {
		return fmt.Errorf("repos: create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := archive.Extract(snap.Path, tmpDir); err != nil {
		return fmt.Errorf("repos: extract snapshot: %w", err)
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
		return fmt.Errorf("repos: no manifest found in snapshot")
	}

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("repos: read manifest: %w", err)
	}

	var manifest reposBackupManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("repos: parse manifest: %w", err)
	}

	for _, repo := range manifest.Repos {
		fmt.Printf("  Repo: %s (%s)\n", repo.Name, repo.Path)
		fmt.Printf("    Remote: %s\n", repo.Remote)
		fmt.Printf("    Branch: %s\n", repo.Branch)
		fmt.Printf("    HEAD: %s\n", repo.HEADHash)
		if repo.Dirty {
			fmt.Printf("    WARNING: Repository had uncommitted changes at backup time\n")
		}
	}

	return nil
}

func (m *ReposModule) Verify(ctx context.Context, snap module.Snapshot) (*module.VerifyResult, error) {
	info, err := os.Stat(snap.Path)
	if err != nil {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{err.Error()}}, nil
	}
	if info.Size() == 0 {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{"empty snapshot"}}, nil
	}
	return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: true}, nil
}

func (m *ReposModule) Doctor(ctx context.Context) (*module.DoctorResult, error) {
	result := &module.DoctorResult{Module: m.Name(), Status: module.DoctorStatusOK}
	home, _ := os.UserHomeDir()

	for _, dir := range devDirs {
		searchPath := filepath.Join(home, dir)
		entries, err := os.ReadDir(searchPath)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			repoPath := filepath.Join(searchPath, entry.Name())
			gitDir := filepath.Join(repoPath, ".git")
			if info, err := os.Stat(gitDir); err != nil || !info.IsDir() {
				continue
			}

			out, err := exec.Command("git", "-C", repoPath, "status", "--porcelain").Output()
			if err == nil && strings.TrimSpace(string(out)) != "" {
				result.Issues = append(result.Issues, module.DoctorIssue{
					Severity: "info",
					Message:  fmt.Sprintf("Repository %q has uncommitted changes", entry.Name()),
					Help:     "Run: git -C " + repoPath + " add && git commit",
				})
			}

			out, err = exec.Command("git", "-C", repoPath, "rev-list", "--count", "--left-right", "HEAD...@{u}").Output()
			if err == nil {
				parts := strings.Fields(string(out))
				if len(parts) == 2 {
					var ahead, behind int
					fmt.Sscanf(parts[0], "%d", &ahead)
					fmt.Sscanf(parts[1], "%d", &behind)
					if ahead > 0 {
						result.Issues = append(result.Issues, module.DoctorIssue{
							Severity: "info",
							Message:  fmt.Sprintf("Repository %q is %d commits ahead of remote", entry.Name(), ahead),
							Help:     "Run: git -C " + repoPath + " push",
						})
					}
					if behind > 0 {
						result.Issues = append(result.Issues, module.DoctorIssue{
							Severity: "info",
							Message:  fmt.Sprintf("Repository %q is %d commits behind remote", entry.Name(), behind),
							Help:     "Run: git -C " + repoPath + " pull",
						})
					}
				}
			}
		}
	}

	if len(result.Issues) > 0 {
		result.Status = module.DoctorStatusWarning
	}
	return result, nil
}

func (m *ReposModule) Dependencies(ctx context.Context) []module.Dependency {
	return []module.Dependency{
		{Type: module.DepSystemPkg, Package: "git", Hint: "Git VCS"},
	}
}

func (m *ReposModule) Install(ctx context.Context, opts module.RestoreOptions) error {
	rt, _ := opts.Runtime.(*runtime.Runtime)
	if rt != nil {
		return rt.Pkg.Install("git")
	}
	return exec.Command("sudo", "apt-get", "install", "-y", "-qq", "git").Run()
}

func (m *ReposModule) Configure(ctx context.Context, opts module.RestoreOptions) error {
	return nil
}

func (m *ReposModule) Validate(ctx context.Context, snap module.Snapshot) (*module.ValidateResult, error) {
	v := restoreutil.NewValidation("repos")

	v.Check(restoreutil.CommandExists("git"), "git installed")

	home := restoreutil.HomeDir()
	for _, dir := range devDirs {
		searchPath := filepath.Join(home, dir)
		if !restoreutil.DirExists(searchPath) {
			continue
		}
		entries, _ := os.ReadDir(searchPath)
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			repoPath := filepath.Join(searchPath, entry.Name())
			if restoreutil.DirExists(filepath.Join(repoPath, ".git")) {
				v.Recovered(fmt.Sprintf("cloned repo: %s", entry.Name()))
			}
		}
	}

	v.Confidence(85)
	return v.Result(), nil
}

func (m *ReposModule) Actions(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) ([]actions.Action, error) {
	home := restoreutil.HomeDir()
	return []actions.Action{
		&actions.ExtractArchive{Source: snap.Path, Destination: home},
	}, nil
}

type restoreUtilAction struct {
	actions.BaseAction
	name string
	desc string
	fn   func(ctx *runtime.RestoreContext) error
}

func (a *restoreUtilAction) Name() string        { return a.name }
func (a *restoreUtilAction) Description() string  { return a.desc }
func (a *restoreUtilAction) Execute(ctx *runtime.RestoreContext) error { return a.fn(ctx) }

var _ actions.Provider = (*ReposModule)(nil)

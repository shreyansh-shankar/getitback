package browserutil

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type InstallMethod string

const (
	InstallApt     InstallMethod = "apt"
	InstallSnap    InstallMethod = "snap"
	InstallFlatpak InstallMethod = "flatpak"
	InstallManual  InstallMethod = "manual"
)

type InstallInfo struct {
	Method    InstallMethod `json:"method"`
	Binary    string        `json:"binary,omitempty"`
	Version   string        `json:"version,omitempty"`
	ConfigDir string        `json:"configDir,omitempty"`
	FlatpakID string        `json:"flatpakID,omitempty"`
}

type BrowserConfig struct {
	Name        string
	Binaries    []string
	SnapName    string
	FlatpakID   string
	ConfigDir   string
}

type Profile struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Default bool   `json:"default"`
}

type ProfileInfo struct {
	Count     int       `json:"count"`
	Profiles  []Profile `json:"profiles,omitempty"`
	Default   string    `json:"default,omitempty"`
	Available bool      `json:"available"`
	Error     string    `json:"error,omitempty"`
}

func DetectInstallation(cfg BrowserConfig) *InstallInfo {
	info := &InstallInfo{ConfigDir: expandHome(cfg.ConfigDir)}

	// Check snap first (most isolated)
	if cfg.SnapName != "" {
		if bin := findSnapBinary(cfg.SnapName, cfg.Binaries); bin != "" {
			info.Method = InstallSnap
			info.Binary = bin
			info.Version = getVersion(bin)
			return info
		}
	}

	// Check flatpak
	if cfg.FlatpakID != "" {
		if ver := flatpakVersion(cfg.FlatpakID); ver != "" {
			info.Method = InstallFlatpak
			info.FlatpakID = cfg.FlatpakID
			info.Version = ver
			return info
		}
	}

	// Check PATH binaries (apt/manual)
	for _, bin := range cfg.Binaries {
		if path, err := exec.LookPath(bin); err == nil {
			info.Method = InstallManual
			info.Binary = path
			info.Version = getVersion(bin)
			return info
		}
	}

	// Check apt dpkg database
	if cfg.Name != "" {
		if ver := dpkgVersion(cfg.Name); ver != "" {
			info.Method = InstallApt
			info.Version = ver
			return info
		}
	}

	// Fallback: config dir exists
	if info.ConfigDir != "" {
		if _, err := os.Stat(info.ConfigDir); err == nil {
			info.Method = InstallManual
			return info
		}
	}

	return nil
}

func findSnapBinary(snapName string, binaries []string) string {
	for _, bin := range binaries {
		p := filepath.Join("/snap/bin", bin)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	p := filepath.Join("/snap/bin", snapName)
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return ""
}

func flatpakVersion(appID string) string {
	out, err := exec.Command("flatpak", "info", "--show-runtime", appID).Output()
	if err != nil {
		return ""
	}
	ver := strings.TrimSpace(string(out))
	if ver != "" {
		return fmt.Sprintf("%s (flatpak)", ver)
	}
	return ""
}

func dpkgVersion(pkgName string) string {
	out, err := exec.Command("dpkg-query", "-W", "-f=${Version}", pkgName).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func getVersion(bin string) string {
	if out, err := exec.Command(bin, "--version").Output(); err == nil {
		return strings.TrimSpace(string(out))
	}
	return ""
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

func DetectChromeProfiles(configDir string) ProfileInfo {
	configDir = expandHome(configDir)
	info := ProfileInfo{Available: false}

	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		info.Error = "config directory does not exist"
		return info
	}

	// Try Local State JSON for canonical profile list
	localState := filepath.Join(configDir, "Local State")
	if data, err := os.ReadFile(localState); err == nil {
		var state struct {
			Profile struct {
				InfoCache map[string]struct {
					Name string `json:"name"`
				} `json:"info_cache"`
			} `json:"profile"`
		}
		if err := json.Unmarshal(data, &state); err == nil && state.Profile.InfoCache != nil {
			for dirName, p := range state.Profile.InfoCache {
				profilePath := filepath.Join(configDir, dirName)
				profile := Profile{
					Name:    p.Name,
					Path:    profilePath,
					Default: dirName == "Default",
				}
				info.Profiles = append(info.Profiles, profile)
				info.Count++
				if profile.Default {
					info.Default = p.Name
				}
				info.Available = true
			}
			if info.Count > 0 {
				return info
			}
		}
	}

	// Fallback: scan directories for Default / Profile N
	entries, err := os.ReadDir(configDir)
	if err != nil {
		info.Error = fmt.Sprintf("cannot read config dir: %v", err)
		info.Available = false
		return info
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if e.Name() == "Default" || strings.HasPrefix(e.Name(), "Profile ") {
			profile := Profile{
				Name:    e.Name(),
				Path:    filepath.Join(configDir, e.Name()),
				Default: e.Name() == "Default",
			}
			info.Profiles = append(info.Profiles, profile)
			info.Count++
			info.Available = true
			if profile.Default {
				info.Default = e.Name()
			}
		}
	}

	if info.Count == 0 {
		info.Error = "directory exists but no profiles found"
		info.Available = true
	}

	return info
}

func firefoxProfilesDir() string {
	home, _ := os.UserHomeDir()
	// Standard path
	d := filepath.Join(home, ".mozilla", "firefox")
	if _, err := os.Stat(d); err == nil {
		return d
	}
	// Snap path
	d = filepath.Join(home, "snap", "firefox", "common", ".mozilla", "firefox")
	if _, err := os.Stat(d); err == nil {
		return d
	}
	return filepath.Join(home, ".mozilla", "firefox")
}

func DetectFirefoxProfiles() ProfileInfo {
	profilesDir := firefoxProfilesDir()
	info := ProfileInfo{Available: false}

	if _, err := os.Stat(profilesDir); os.IsNotExist(err) {
		info.Error = "firefox config directory does not exist"
		return info
	}

	// Parse profiles.ini
	seenPaths := make(map[string]bool)
	iniPath := filepath.Join(profilesDir, "profiles.ini")
	if data, err := os.ReadFile(iniPath); err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		var currentSection string
		var currentProfile Profile
		inProfile := false

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
				continue
			}

			if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
				if inProfile && currentProfile.Name != "" {
					info.Profiles = append(info.Profiles, currentProfile)
					seenPaths[currentProfile.Path] = true
				}
				currentSection = line[1 : len(line)-1]
				inProfile = strings.HasPrefix(currentSection, "Profile")
				if inProfile {
					currentProfile = Profile{
						Path: filepath.Join(profilesDir),
					}
				}
				continue
			}

			if !inProfile {
				continue
			}

			if parts := strings.SplitN(line, "=", 2); len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])
				switch key {
				case "Name":
					currentProfile.Name = val
				case "Path":
					if !filepath.IsAbs(val) {
						currentProfile.Path = filepath.Join(profilesDir, val)
					} else {
						currentProfile.Path = val
					}
				case "Default":
					currentProfile.Default = val == "1"
					if currentProfile.Default {
						info.Default = currentProfile.Name
					}
				}
			}
		}

		if inProfile && currentProfile.Name != "" {
			info.Profiles = append(info.Profiles, currentProfile)
			seenPaths[currentProfile.Path] = true
		}
	}

	// Scan directory for any additional profile dirs not in profiles.ini
	entries, err := os.ReadDir(profilesDir)
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			if !strings.Contains(e.Name(), ".") {
				continue
			}
			profilePath := filepath.Join(profilesDir, e.Name())
			if seenPaths[profilePath] {
				continue
			}
			parts := strings.SplitN(e.Name(), ".", 2)
			p := Profile{
				Name:    parts[1],
				Path:    profilePath,
				Default: info.Default == "" && info.Count == 0,
			}
			info.Profiles = append(info.Profiles, p)
			if p.Default {
				info.Default = p.Name
			}
		}
	}

	info.Count = len(info.Profiles)
	info.Available = true

	if info.Count == 0 {
		info.Error = "no firefox profiles found"
	}

	return info
}

func DirSize(path string) int64 {
	var size int64
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}

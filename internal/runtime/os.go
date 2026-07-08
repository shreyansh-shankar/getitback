package runtime

import (
	"os"
	"runtime"
	"strings"
)

type OSInfo struct {
	ID        string // "ubuntu", "debian", "fedora", etc.
	Version   string
	Arch      string
	IsRoot    bool
	HomeDir   string
}

func DetectOS() OSInfo {
	info := OSInfo{
		Arch:    runtime.GOARCH,
		IsRoot:  os.Geteuid() == 0,
		HomeDir: os.Getenv("HOME"),
	}
	if info.HomeDir == "" {
		info.HomeDir = "/root"
	}

	data, err := os.ReadFile("/etc/os-release")
	if err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "ID=") {
				info.ID = strings.Trim(strings.TrimPrefix(line, "ID="), "\"")
			}
			if strings.HasPrefix(line, "VERSION_ID=") {
				info.Version = strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), "\"")
			}
		}
	}
	if info.ID == "" {
		info.ID = "linux"
	}

	return info
}

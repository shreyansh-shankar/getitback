package env

import (
	"os"
	"strings"
)

type EnvManager struct {
	profilePath string
	exports     map[string]string
}

func NewEnvManager() EnvManager {
	home := os.Getenv("HOME")
	if home == "" {
		home = "/root"
	}

	profile := home + "/.profile"
	if _, err := os.Stat(home + "/.bashrc"); err == nil {
		profile = home + "/.bashrc"
	}

	return EnvManager{
		profilePath: profile,
		exports:     make(map[string]string),
	}
}

func (e EnvManager) Set(key, value string) error {
	return os.Setenv(key, value)
}

func (e EnvManager) Get(key string) string {
	return os.Getenv(key)
}

func (e EnvManager) Unset(key string) error {
	return os.Unsetenv(key)
}

func (e EnvManager) AppendToPath(dir string) error {
	path := os.Getenv("PATH")
	if path == "" {
		return os.Setenv("PATH", dir)
	}
	if !strings.Contains(path, dir) {
		return os.Setenv("PATH", path+":"+dir)
	}
	return nil
}

func (e EnvManager) ExportToProfile(key, value string) error {
	e.exports[key] = value
	data := []byte("\nexport " + key + "=" + value + "\n")

	existing, err := os.ReadFile(e.profilePath)
	if err != nil {
		return os.WriteFile(e.profilePath, data, 0644)
	}

	if strings.Contains(string(existing), "export "+key+"=") {
		return nil
	}

	f, err := os.OpenFile(e.profilePath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(data)
	return err
}

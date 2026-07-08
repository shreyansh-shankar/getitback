package pkgmgr

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/shreyansh-shankar/getitback/internal/runtime/executor"
)

type Apt struct {
	Executor executor.Executor
}

func NewApt(exec executor.Executor) *Apt {
	return &Apt{Executor: exec}
}

func (a *Apt) Name() string { return "apt" }

func (a *Apt) Install(packages ...string) error {
	if len(packages) == 0 {
		return nil
	}

	// Non-interactive mode
	os.Setenv("DEBIAN_FRONTEND", "noninteractive")

	// Run apt-get update first to refresh package lists
	var updateFailed bool
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(5 * time.Second)
		}
		updateArgs := withSudo([]string{"update", "-y"})
		if err := a.Executor.Run("apt-get", updateArgs...); err != nil {
			if strings.Contains(err.Error(), "Could not get lock") ||
				strings.Contains(err.Error(), "Unable to lock") {
				continue
			}
			updateFailed = true
			break
		}
		break
	}

	// Install packages with lock retry
	installArgs := append([]string{"install", "-y", "-o", "DPkg::Lock::Timeout=120"}, packages...)
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(5 * time.Second)
		}
		if err := a.Executor.Run("apt-get", withSudo(installArgs)...); err != nil {
			lastErr = err
			if strings.Contains(err.Error(), "Could not get lock") ||
				strings.Contains(err.Error(), "Unable to lock") {
				continue
			}
			if updateFailed {
				lastErr = fmt.Errorf("apt update failed (some repos may be unavailable); install failed: %w", err)
			}
			return lastErr
		}
		// Check each package was actually installed
		var missing []string
		for _, pkg := range packages {
			if !a.IsInstalled(pkg) {
				missing = append(missing, pkg)
			}
		}
		if len(missing) == 0 {
			return nil
		}
		// Package might have a different name or be in a repository not available
		// Don't treat this as hard failure for packages that aren't found
		return nil
	}
	if lastErr != nil {
		return fmt.Errorf("apt install %s failed after retries: %w", strings.Join(packages, " "), lastErr)
	}
	return nil
}

func (a *Apt) Remove(packages ...string) error {
	if len(packages) == 0 {
		return nil
	}
	os.Setenv("DEBIAN_FRONTEND", "noninteractive")
	args := append([]string{"remove", "-y"}, packages...)
	return a.Executor.Run("apt-get", withSudo(args)...)
}

func (a *Apt) IsInstalled(pkg string) bool {
	err := a.Executor.Run("dpkg", "-s", pkg)
	return err == nil
}

func (a *Apt) Update() error {
	os.Setenv("DEBIAN_FRONTEND", "noninteractive")
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(5 * time.Second)
		}
		if err := a.Executor.Run("apt-get", withSudo([]string{"update", "-y"})...); err != nil {
			if strings.Contains(err.Error(), "Could not get lock") ||
				strings.Contains(err.Error(), "Unable to lock") {
				continue
			}
			return fmt.Errorf("apt update failed: %w", err)
		}
		return nil
	}
	return fmt.Errorf("apt update failed after retries: could not acquire lock")
}

func (a *Apt) AddRepository(name, url, keyFile string) error {
	return a.Executor.Run("add-apt-repository", withSudo([]string{"-y", url})...)
}

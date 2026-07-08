package restoreutil

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/shreyansh-shankar/getitback/internal/module"
)

type ValidationBuilder struct {
	module.ValidateResult
}

func NewValidation(name string) *ValidationBuilder {
	return &ValidationBuilder{
		ValidateResult: module.ValidateResult{
			Module:  name,
			Success: true,
		},
	}
}

func (v *ValidationBuilder) Check(ok bool, msg string, args ...any) *ValidationBuilder {
	if ok {
		v.Checks = append(v.Checks, fmt.Sprintf(msg, args...))
	} else {
		v.Success = false
		v.Errors = append(v.Errors, fmt.Sprintf(msg, args...))
	}
	return v
}

func (v *ValidationBuilder) Warn(format string, args ...any) *ValidationBuilder {
	v.Warnings = append(v.Warnings, fmt.Sprintf(format, args...))
	return v
}

func (v *ValidationBuilder) Error(format string, args ...any) *ValidationBuilder {
	v.Success = false
	v.Errors = append(v.Errors, fmt.Sprintf(format, args...))
	return v
}

func (v *ValidationBuilder) Manual(format string, args ...any) *ValidationBuilder {
	v.ManualSteps = append(v.ManualSteps, fmt.Sprintf(format, args...))
	return v
}

func (v *ValidationBuilder) Recovered(asset string) *ValidationBuilder {
	v.ValidateResult.Recovered = append(v.ValidateResult.Recovered, asset)
	return v
}

func (v *ValidationBuilder) Missing(asset string) *ValidationBuilder {
	v.ValidateResult.Missing = append(v.ValidateResult.Missing, asset)
	return v
}

func (v *ValidationBuilder) Version(ver string) *ValidationBuilder {
	v.ValidateResult.Version = ver
	return v
}

func (v *ValidationBuilder) Confidence(score int) *ValidationBuilder {
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	v.ValidateResult.Confidence = score
	return v
}

func (v *ValidationBuilder) Result() *module.ValidateResult {
	score := 100
	if len(v.ValidateResult.Errors) > 0 {
		score -= len(v.ValidateResult.Errors) * 20
	}
	if len(v.ValidateResult.Missing) > 0 {
		score -= len(v.ValidateResult.Missing) * 10
	}
	if v.ValidateResult.Confidence == 0 {
		v.ValidateResult.Confidence = score
		if v.ValidateResult.Confidence < 0 {
			v.ValidateResult.Confidence = 0
		}
	}
	v.ValidateResult.Success = v.ValidateResult.Success && len(v.ValidateResult.Errors) == 0
	return &v.ValidateResult
}

func CommandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func DirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func HomeDir() string {
	home := os.Getenv("HOME")
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	return home
}

func ValidateCommand(name string) error {
	if !CommandExists(name) {
		return fmt.Errorf("%s not found in PATH", name)
	}
	return nil
}

func ValidateEnvVar(key string) (string, error) {
	val := os.Getenv(key)
	if val == "" {
		return "", fmt.Errorf("environment variable %s is not set", key)
	}
	return val, nil
}

func ValidateFile(path string) error {
	if !FileExists(path) {
		return fmt.Errorf("file not found: %s", path)
	}
	return nil
}

func ValidateDir(path string) error {
	if !DirExists(path) {
		return fmt.Errorf("directory not found: %s", path)
	}
	return nil
}

func CheckExecOutput(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s: %w", strings.Join(append([]string{name}, args...), " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

func CheckExec(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	return cmd.Run()
}



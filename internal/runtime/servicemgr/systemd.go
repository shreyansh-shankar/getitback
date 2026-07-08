package servicemgr

import (
	"strings"

	"github.com/shreyansh-shankar/getitback/internal/runtime/executor"
)

type Systemd struct {
	exec executor.Executor
}

func NewSystemd(e executor.Executor) *Systemd {
	return &Systemd{exec: e}
}

func (s *Systemd) Name() string { return "systemd" }

func (s *Systemd) Enable(name string) error {
	return s.exec.Run("systemctl", s.args("enable", name)...)
}

func (s *Systemd) Disable(name string) error {
	return s.exec.Run("systemctl", s.args("disable", name)...)
}

func (s *Systemd) Start(name string) error {
	return s.exec.Run("systemctl", s.args("start", name)...)
}

func (s *Systemd) Stop(name string) error {
	return s.exec.Run("systemctl", s.args("stop", name)...)
}

func (s *Systemd) Restart(name string) error {
	return s.exec.Run("systemctl", s.args("restart", name)...)
}

func (s *Systemd) Status(name string) (bool, error) {
	res := s.exec.RunCapture("systemctl", append(s.prefix(), "is-active", name)...)
	if res.Err != nil {
		return false, nil
	}
	return res.Stdout == "active\n", nil
}

func (s *Systemd) Exists(name string) bool {
	res := s.exec.RunCapture("systemctl", append(s.prefix(), "list-units", "--type=service", "--all", "--no-legend")...)
	if res.Err != nil {
		return false
	}
	for _, line := range strings.Split(res.Stdout, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && strings.HasPrefix(fields[0], name) {
			return true
		}
	}
	// Also check unit files
	res2 := s.exec.RunCapture("systemctl", append(s.prefix(), "list-unit-files", "--type=service", "--no-legend")...)
	if res2.Err != nil {
		return false
	}
	for _, line := range strings.Split(res2.Stdout, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && strings.HasPrefix(fields[0], name) {
			return true
		}
	}
	return false
}

func (s *Systemd) Reload() error {
	return s.exec.Run("systemctl", append(s.prefix(), "daemon-reload")...)
}

func (s *Systemd) prefix() []string {
	if executor.IsRoot() {
		return nil
	}
	return []string{"sudo"}
}

func (s *Systemd) args(action, unit string) []string {
	return append(s.prefix(), action, unit)
}

var _ ServiceManager = (*Systemd)(nil)

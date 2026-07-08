package executor

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
)

type Options struct {
	Prefix string
	Logger *slog.Logger
}

type CmdResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
	Err      error
}

func (r CmdResult) Success() bool {
	return r.Err == nil
}

type Command interface {
	Run() error
	Output() ([]byte, error)
	CombinedOutput() ([]byte, error)
	String() string
}

type cmdWrapper struct {
	*exec.Cmd
	prefix string
}

func (c *cmdWrapper) String() string {
	return strings.Join(c.Args, " ")
}

type Executor struct {
	opts Options
}

func NewExecutor(opts Options) Executor {
	if opts.Logger == nil {
		opts.Logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	}
	return Executor{opts: opts}
}

func (e Executor) Command(name string, args ...string) Command {
	cmd := exec.Command(name, args...)
	return &cmdWrapper{Cmd: cmd, prefix: e.opts.Prefix}
}

func (e Executor) Run(name string, args ...string) error {
	cmd := e.Command(name, args...)
	e.opts.Logger.Debug("exec", "cmd", cmd.String())
	return cmd.Run()
}

func (e Executor) Output(name string, args ...string) (string, error) {
	cmd := e.Command(name, args...)
	e.opts.Logger.Debug("exec", "cmd", cmd.String())
	out, err := cmd.Output()
	return string(out), err
}

func (e Executor) RunCapture(name string, args ...string) CmdResult {
	cmd := e.Command(name, args...)
	stdout, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return CmdResult{
				ExitCode: exitErr.ExitCode(),
				Stderr:   string(exitErr.Stderr),
				Err:      fmt.Errorf("%s: %s", strings.Join(append([]string{name}, args...), " "), string(exitErr.Stderr)),
			}
		}
		return CmdResult{Err: err}
	}
	return CmdResult{Stdout: string(stdout)}
}

func (e Executor) RunBash(script string) error {
	cmd := e.Command("bash", "-c", script)
	e.opts.Logger.Debug("exec", "cmd", cmd.String())
	return cmd.Run()
}

func IsRoot() bool {
	return os.Geteuid() == 0
}

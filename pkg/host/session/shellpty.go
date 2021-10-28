package session

import (
	"github.com/cirruslabs/terminal/internal/api"
	"github.com/creack/pty"
	"go.uber.org/zap"
	"os"
	"os/exec"
)

type ShellPTY struct {
	logger   *zap.SugaredLogger
	shellCmd *exec.Cmd
	pty      *os.File
}

func NewShellPTY(logger *zap.SugaredLogger, dimensions *api.TerminalDimensions) (*ShellPTY, error) {
	// Create a PTY with a shell attached to it
	shellPath := determineShellPath()
	shellCmd := exec.Command(shellPath)

	// Inherit this process environment variables
	shellCmd.Env = os.Environ()

	// Set TERM to avoid "Error opening terminal: unknown." error
	shellCmd.Env = append(shellCmd.Env, "TERM=xterm")

	pty, err := pty.StartWithSize(shellCmd, terminalDimensionsToPtyWinsize(dimensions))
	if err != nil {
		return nil, err
	}

	logger.Debugf("started shell process with PID %d", shellCmd.Process.Pid)

	return &ShellPTY{
		logger:   logger,
		shellCmd: shellCmd,
		pty:      pty,
	}, nil
}

func (sp *ShellPTY) Write(b []byte) (n int, err error) {
	return sp.pty.Write(b)
}

func (sp *ShellPTY) Read(b []byte) (n int, err error) {
	return sp.pty.Read(b)
}

func (sp *ShellPTY) Resize(dimensions *api.TerminalDimensions) error {
	return pty.Setsize(sp.pty, terminalDimensionsToPtyWinsize(dimensions))
}

func (sp *ShellPTY) Close() error {
	var result error

	if err := sp.pty.Close(); err != nil {
		result = err
	}

	sp.logger.Debugf("killing shell process with PID %d", sp.shellCmd.Process.Pid)

	if err := sp.shellCmd.Process.Kill(); err != nil {
		sp.logger.Warnf("failed to kill shell process with PID %d: %v", sp.shellCmd.Process.Pid, err)

		if result == nil {
			result = err
		}
	}

	if err := sp.shellCmd.Wait(); err != nil && result == nil {
		result = err
	}

	return result
}

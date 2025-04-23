//go:build !windows
// +build !windows

package session

import (
	"context"
	"errors"
	"github.com/cirruslabs/terminal/internal/api"
	"github.com/creack/pty"
	"go.uber.org/zap"
	"io"
	"math"
	"os/exec"
	"runtime"
	"sync"
	"time"
)

const (
	defaultWidthColumns = 80
	defaultHeightRows   = 24
)

type Session struct {
	logger *zap.SugaredLogger

	token string

	shellEnv []string

	lastActivityLock sync.Mutex
	lastActivity     time.Time
}

func New(logger *zap.Logger, token string, shellEnv []string) *Session {
	return &Session{
		logger:   logger.Sugar(),
		token:    token,
		shellEnv: shellEnv,
	}
}

func (session *Session) Token() string {
	return session.token
}

func (session *Session) Run(
	ctx context.Context,
	hostService api.HostServiceClient,
	locator string,
	dimensions *api.TerminalDimensions,
) {
	dataChannelCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	dataChannel, err := hostService.DataChannel(dataChannelCtx)
	if err != nil {
		session.logger.Warnf("failed to open data channel: %v", err)
		return
	}

	if err := dataChannel.Send(&api.HostDataRequest{
		Operation: &api.HostDataRequest_Hello_{
			Hello: &api.HostDataRequest_Hello{
				Locator: locator,
				Token:   session.Token(),
			},
		},
	}); err != nil {
		session.logger.Warnf("failed to send Hello message via data channel: %v", err)
		return
	}

	shellPty, err := NewShellPTY(session.logger, dimensions, session.shellEnv)
	if err != nil {
		session.logger.Warnf("failed to create PTY with shell: %v", err)
		return
	}

	// Ensure we cleanup both the PTY and the created shell process
	defer func() {
		if err := shellPty.Close(); err != nil {
			session.logger.Warnf("failed to close PTY with shell: %v", err)
		}
	}()

	// Receive terminal input from the server and write it to the PTY
	go func() {
		defer cancel()
		session.ioToPty(dataChannel, shellPty)
	}()

	// Read output from the PTY and send it to the server
	go func() {
		defer cancel()
		session.ioFromPty(dataChannel, shellPty)
	}()

	<-dataChannelCtx.Done()
}

func (session *Session) ioToPty(dataChannel api.HostService_DataChannelClient, shellPty *ShellPTY) {
	for {
		dataFromServer, err := dataChannel.Recv()
		if err != nil {
			if !errors.Is(err, io.EOF) && dataChannel.Context().Err() == nil {
				session.logger.Warnf("failed to receive Data message from data channel: %v", err)
			}

			return
		}

		session.updateLastActivity()

		switch op := dataFromServer.Operation.(type) {
		case *api.HostDataResponse_Input:
			if _, err := shellPty.Write(op.Input.Data); err != nil {
				session.logger.Warnf("failed to write to PTY: %v", err)
				return
			}
		case *api.HostDataResponse_ChangeDimensions:
			if err := shellPty.Resize(op.ChangeDimensions); err != nil {
				session.logger.Warnf("failed to resize PTY: %v", err)
				return
			}
		default:
			session.logger.Warnf("should've received a Data or a ChangeDimensions message")
			return
		}
	}
}

func (session *Session) ioFromPty(dataChannel api.HostService_DataChannelClient, shellPty io.Reader) {
	const bufSize = 4096
	buf := make([]byte, bufSize)

	for {
		n, err := shellPty.Read(buf)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				session.logger.Warnf("failed to read data from the PTY: %v", err)
			}

			return
		}

		if err := dataChannel.Send(&api.HostDataRequest{
			Operation: &api.HostDataRequest_Output{
				Output: &api.Data{
					Data: buf[:n],
				},
			},
		}); err != nil {
			if !errors.Is(err, io.EOF) && dataChannel.Context().Err() == nil {
				session.logger.Warnf("failed to send data from PTY: %v", err)
			}

			return
		}
	}
}

func (session *Session) LastActivity() time.Time {
	session.lastActivityLock.Lock()
	defer session.lastActivityLock.Unlock()

	return session.lastActivity
}

func (session *Session) updateLastActivity() {
	session.lastActivityLock.Lock()
	defer session.lastActivityLock.Unlock()

	now := time.Now()
	if now.After(session.lastActivity) {
		session.lastActivity = now
	}
}

func determineShellPath() string {
	shellPath := "/bin/sh"

	// Prefer Zsh on macOS
	if runtime.GOOS == "darwin" {
		if zshPath, err := exec.LookPath("zsh"); err == nil {
			return zshPath
		}
	}

	if bashPath, err := exec.LookPath("bash"); err == nil {
		shellPath = bashPath
	}

	return shellPath
}

func terminalDimensionsToPtyWinsize(terminalDimensions *api.TerminalDimensions) *pty.Winsize {
	if terminalDimensions == nil {
		return &pty.Winsize{
			Cols: defaultWidthColumns,
			Rows: defaultHeightRows,
		}
	}

	return &pty.Winsize{
		Cols: safeUint32ToUint16(terminalDimensions.WidthColumns, defaultWidthColumns),
		Rows: safeUint32ToUint16(terminalDimensions.HeightRows, defaultHeightRows),
	}
}

func safeUint32ToUint16(val uint32, fallback uint16) uint16 {
	if val > math.MaxUint16 {
		return fallback
	}

	return uint16(val)
}

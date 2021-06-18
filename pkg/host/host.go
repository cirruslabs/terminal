// +build !windows

package host

import (
	"context"
	"errors"
	"fmt"
	"github.com/cirruslabs/terminal/internal/api"
	"github.com/creack/pty"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"io"
	"os"
	"os/exec"
	"time"
)

const (
	defaultServerAddress = "terminal.cirrus-ci.com"
	defaultWidthColumns  = 80
	defaultHeightRows    = 24
)

var (
	ErrProtocol = errors.New("protocol error")
	ErrSecurity = errors.New("security violation")
)

func New(opts ...Option) (*TerminalHost, error) {
	client := &TerminalHost{}

	// Apply options
	for _, opt := range opts {
		opt(client)
	}

	// Apply defaults
	if client.logger == nil {
		client.logger = logrus.New()
		client.logger.Out = io.Discard
	}
	if client.serverAddress == "" {
		client.serverAddress = defaultServerAddress
	}

	// Sanity check
	if client.trustedSecret == "" {
		return nil, fmt.Errorf("%w: empty trusted secret supplied", ErrSecurity)
	}

	return client, nil
}

func (th *TerminalHost) Run(ctx context.Context) error {
	var dialOpts []grpc.DialOption

	if th.serverInsecure {
		dialOpts = append(dialOpts, grpc.WithInsecure())
	}

	clientConn, err := grpc.Dial(th.serverAddress, dialOpts...)
	if err != nil {
		return err
	}

	hostService := api.NewHostServiceClient(clientConn)

	controlChannel, err := hostService.ControlChannel(ctx)
	if err != nil {
		return err
	}

	// Send Hello
	err = controlChannel.Send(&api.HostControlRequest{
		Operation: &api.HostControlRequest_Hello_{
			Hello: &api.HostControlRequest_Hello{
				TrustedSecret: th.trustedSecret,
			},
		},
	})
	if err != nil {
		return err
	}

	// Receive Hello
	controlFromServer, err := controlChannel.Recv()
	if err != nil {
		return err
	}
	helloFromServer := controlFromServer.GetHello()
	if helloFromServer == nil {
		return fmt.Errorf("%w: should've received a Hello message", ErrProtocol)
	}

	if th.locatorCallback != nil {
		th.locatorCallback(helloFromServer.Locator)
	}

	// Loop waiting for the data channels to be requested
	for {
		controlFromServer, err = controlChannel.Recv()
		if err != nil {
			select {
			case <-controlChannel.Context().Done():
				return controlChannel.Context().Err()
			default:
				return err
			}
		}
		dataChannelRequest := controlFromServer.GetDataChannelRequest()
		if dataChannelRequest == nil {
			return fmt.Errorf("%w: should've received a DataChannelRequest message", ErrProtocol)
		}

		go th.launchDataChannel(ctx, hostService, helloFromServer.Locator, dataChannelRequest)
	}
}

func (th *TerminalHost) LastActivity() time.Time {
	th.lastActivityLock.Lock()
	defer th.lastActivityLock.Unlock()

	return th.lastActivity
}

func (th *TerminalHost) updateLastActivity() {
	th.lastActivityLock.Lock()
	defer th.lastActivityLock.Unlock()

	now := time.Now()
	if now.After(th.lastActivity) {
		th.lastActivity = now
	}
}

func (th *TerminalHost) launchDataChannel(
	ctx context.Context,
	hostService api.HostServiceClient,
	locator string,
	dataChannelRequest *api.HostControlResponse_DataChannelRequest,
) {
	th.updateLastActivity()

	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	dataChannel, err := hostService.DataChannel(subCtx)
	if err != nil {
		th.logger.Warnf("failed to open data channel: %v", err)
		return
	}

	if err := dataChannel.Send(&api.HostDataRequest{
		Operation: &api.HostDataRequest_Hello_{
			Hello: &api.HostDataRequest_Hello{
				Locator: locator,
				Token:   dataChannelRequest.Token,
			},
		},
	}); err != nil {
		th.logger.Warnf("failed to send Hello message via data channel: %v", err)
		return
	}

	// Create a PTY with a shell attached to it
	shellPath := determineShellPath()
	shellCmd := exec.Command(shellPath)

	shellPty, err := pty.StartWithSize(shellCmd, terminalDimensionsToPtyWinsize(dataChannelRequest.RequestedDimensions))
	if err != nil {
		th.logger.Warnf("failed to start PTY: %v", err)
	}

	th.logger.Debugf("started shell process with PID %d", shellCmd.Process.Pid)

	// Ensure we cleanup both the PTY and the created shell process
	defer func() {
		if err := shellPty.Close(); err != nil {
			th.logger.Warnf("failed to close PTY: %v", err)
		}

		th.logger.Debugf("killing shell process with PID %d", shellCmd.Process.Pid)

		if err := shellCmd.Process.Kill(); err != nil {
			th.logger.Warnf("failed to kill shell process with PID %d: %v", shellCmd.Process.Pid, err)
		}

		_ = shellCmd.Wait()
	}()

	// Receive terminal input from the server and write it to the PTY
	go func() {
		defer cancel()
		th.ioToPty(dataChannel, shellPty)
	}()

	// Read output from the PTY and send it to the server
	go func() {
		defer cancel()
		th.ioFromPty(dataChannel, shellPty)
	}()

	<-subCtx.Done()
}

func (th *TerminalHost) ioToPty(dataChannel api.HostService_DataChannelClient, shellPty *os.File) {
	for {
		th.updateLastActivity()

		dataFromServer, err := dataChannel.Recv()
		if err != nil {
			select {
			case <-dataChannel.Context().Done():
				// ignore
			default:
				th.logger.Warnf("failed to receive Data message from data channel: %v", err)
			}

			return
		}

		switch op := dataFromServer.Operation.(type) {
		case *api.HostDataResponse_Input:
			if _, err := shellPty.Write(op.Input.Data); err != nil {
				th.logger.Warnf("failed to write to PTY: %v", err)
				return
			}
		case *api.HostDataResponse_ChangeDimensions:
			if err := pty.Setsize(shellPty, terminalDimensionsToPtyWinsize(op.ChangeDimensions)); err != nil {
				th.logger.Warnf("failed to resize PTY: %v", err)
				return
			}
		default:
			th.logger.Warnf("should've received a Data or a ChangeDimensions message")
			return
		}
	}
}

func (th *TerminalHost) ioFromPty(dataChannel api.HostService_DataChannelClient, shellPty io.Reader) {
	const bufSize = 4096
	buf := make([]byte, bufSize)

	for {
		th.updateLastActivity()

		n, err := shellPty.Read(buf)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				th.logger.Warnf("failed to read data from the PTY: %v", err)
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
			if !errors.Is(err, io.EOF) {
				th.logger.Warnf("failed to send data from PTY: %v", err)
			}

			return
		}
	}
}

func determineShellPath() string {
	shellPath := "/bin/sh"

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
		Cols: uint16(terminalDimensions.WidthColumns),
		Rows: uint16(terminalDimensions.HeightRows),
	}
}

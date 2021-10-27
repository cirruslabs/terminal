package server

import (
	"github.com/cirruslabs/terminal/internal/api"
	"github.com/cirruslabs/terminal/internal/server/terminal"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (ts *TerminalServer) ControlChannel(channel api.HostService_ControlChannelServer) error {
	logger := ts.logger.With(ts.TraceContext(channel.Context())...)

	// Host begins with sending a Hello message that contains the credentials it trusts
	requestFromHost, err := channel.Recv()
	if err != nil {
		logger.Warn("failed to receive a Hello message", zap.Error(err))
		return err
	}
	helloFromHost := requestFromHost.GetHello()
	if helloFromHost == nil {
		logger.Warn("expected a Hello message, got something else")
		return status.Errorf(codes.FailedPrecondition, "expected a Hello message")
	}

	// Create and register a new terminal associated with this Host
	terminal := terminal.New(ts.generateLocator(), terminal.WithTrustedSecret(helloFromHost.TrustedSecret))
	defer terminal.Close()

	logger = logger.With(LocatorField(terminal.Locator()))

	if err := ts.registerTerminal(terminal); err != nil {
		logger.Warn("failed to register terminal", zap.Error(err))
		return err
	}
	defer ts.unregisterTerminal(terminal)

	logger.Info("registered new terminal")

	// Tell the Host it's locator
	if err := channel.Send(&api.HostControlResponse{
		Operation: &api.HostControlResponse_Hello_{
			Hello: &api.HostControlResponse_Hello{
				Locator: terminal.Locator(),
			},
		},
	}); err != nil {
		logger.Warn("failed tell the host it's locator", zap.Error(err))
		return err
	}

	for {
		select {
		case session := <-terminal.NewSessionChan:
			logger = logger.With(HashedTokenField(session.Token()))

			// There's a new session created for this terminal on the Guest's side,
			// commence the I/O proxying process by telling the Host to open a new data channel
			if err := channel.Send(&api.HostControlResponse{
				Operation: &api.HostControlResponse_DataChannelRequest_{
					DataChannelRequest: &api.HostControlResponse_DataChannelRequest{
						Token:               session.Token(),
						RequestedDimensions: session.RequestedDimensions(),
					},
				},
			}); err != nil {
				logger.Warn("failed to tell the host about the new session")
				return err
			}

			logger.Info("spawned new session")
		case <-channel.Context().Done():
			// The Host has left and there's nothing we can do about it except close and unregister it's terminal
			logger.Info("host has disconnected", zap.Error(channel.Context().Err()))
			return nil
		}
	}
}

func (ts *TerminalServer) DataChannel(channel api.HostService_DataChannelServer) error {
	logger := ts.logger.With(ts.TraceContext(channel.Context())...)

	// Host begins the channel by sending a Hello message
	// with the token it received from the control channel
	requestFromHost, err := channel.Recv()
	if err != nil {
		logger.Warn("failed to receive a Hello message", zap.Error(err))
		return err
	}
	helloFromHost := requestFromHost.GetHello()
	if helloFromHost == nil {
		logger.Warn("expected a Hello message, got something else")
		return status.Errorf(codes.FailedPrecondition, "expected a Hello message")
	}

	logger = logger.With(LocatorField(helloFromHost.Locator), HashedTokenField(helloFromHost.Token))

	terminal := ts.findTerminal(helloFromHost.Locator)
	if terminal == nil {
		logger.Warn("terminal with the specified locator not found")
		return status.Errorf(codes.NotFound, "terminal with locator %q not found", helloFromHost.Locator)
	}

	session := terminal.FindSession(helloFromHost.Token)
	if session == nil {
		logger.Warn("terminal has no active sessions with the specified token")
		return status.Errorf(codes.NotFound, "terminal %q has no active sessions with the specified token",
			terminal.Locator())
	}

	logger.Info("established new terminal session")

	// A way to terminate channel if we receive at least one error from one of the two Goroutines below
	const numGoroutines = 2
	errChan := make(chan error, numGoroutines)

	// Process terminal input and other commands from the Guest
	go func() {
		for {
			var responseToHost *api.HostDataResponse

			select {
			case chunk := <-session.TerminalInputChan:
				responseToHost = &api.HostDataResponse{
					Operation: &api.HostDataResponse_Input{
						Input: &api.Data{
							Data: chunk,
						},
					},
				}
			case newDimensions := <-session.ChangeDimensionsChan:
				responseToHost = &api.HostDataResponse{
					Operation: &api.HostDataResponse_ChangeDimensions{
						ChangeDimensions: newDimensions,
					},
				}
			case <-channel.Context().Done():
				logger.Warn("terminal channel was closed by the host", zap.Error(channel.Context().Err()))
				errChan <- nil
				return
			case <-session.Context().Done():
				logger.Warn("terminal channel was closed by the guest")
				errChan <- status.Errorf(codes.Aborted, "terminal channel was closed by the guest")
				return
			}

			if err := channel.Send(responseToHost); err != nil {
				logger.Warn("failed to tell the host about guest's input/commands", zap.Error(err))
				errChan <- err
				return
			}
		}
	}()

	// Process terminal output from the Host
	go func() {
		for {
			requestFromHost, err := channel.Recv()
			if err != nil {
				logger.Warn("failed to receive terminal output from the host", zap.Error(err))
				errChan <- err
				return
			}
			outputFromHost := requestFromHost.GetOutput()
			if outputFromHost == nil {
				logger.Warn("expected a Data message from the host, got something else")
				errChan <- status.Errorf(codes.FailedPrecondition, "expected a Data message")
				return
			}

			select {
			case session.TerminalOutputChan <- outputFromHost.Data:
				continue
			case <-channel.Context().Done():
				logger.Warn("terminal channel was closed by the host", zap.Error(channel.Context().Err()))
				errChan <- nil
				return
			case <-session.Context().Done():
				errChan <- status.Errorf(codes.Aborted, "terminal channel was closed by the guest")
				return
			}
		}
	}()

	return <-errChan
}

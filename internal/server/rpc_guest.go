package server

import (
	"github.com/cirruslabs/terminal/internal/api"
	"github.com/cirruslabs/terminal/internal/server/session"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (ts *TerminalServer) TerminalChannel(channel api.GuestService_TerminalChannelServer) error {
	logger := ts.logger.With(ts.TraceContext(channel.Context())...)

	// Guest begins the session by sending a Hello message
	// with the credentials of the terminal it wants to talk to
	requestFromGuest, err := channel.Recv()
	if err != nil {
		logger.Warn("failed to receive a Hello message")
		return err
	}
	helloFromGuest := requestFromGuest.GetHello()
	if helloFromGuest == nil {
		logger.Warn("expected a Hello message, got something else")
		return status.Errorf(codes.FailedPrecondition, "expected a Hello message")
	}

	logger = logger.With(LocatorField(helloFromGuest.Locator), HashedSecretField(helloFromGuest.Secret))

	// Find a terminal with the requested locator
	terminal := ts.findTerminal(helloFromGuest.Locator)
	if terminal == nil {
		logger.Warn("terminal with the specified locator is not registered on this server")
		return status.Errorf(codes.NotFound, "terminal with locator %q is not registered on this server",
			helloFromGuest.Locator)
	}

	// Authenticate the Guest
	if !terminal.IsSecretValid(helloFromGuest.Secret) {
		logger.Warn("guest provided an invalid secret")
		return status.Errorf(codes.PermissionDenied, "invalid secret")
	}

	// Start a new session on this terminal
	session := session.New(channel.Context(), helloFromGuest.RequestedDimensions)
	defer session.Close()

	logger = logger.With(HashedTokenField(session.Token()))

	if err := terminal.RegisterSession(session); err != nil {
		logger.Warn("failed to register a new session", zap.Error(err))
		return err
	}
	defer terminal.UnregisterSession(session)

	logger.Info("started a new session")

	// Broadcast the created session
	select {
	case terminal.NewSessionChan <- session:
		// OK, proceed with session I/O below
	case <-channel.Context().Done():
		// Connection with the Guest was terminated before the Host had a chance to pick up our session
		logger.Warn("connection with the guest terminated before the host had a chance to pick up the session")
		return nil
	}

	// A way to terminate channel if we receive at least one error from one of the two Goroutines below
	const numGoroutines = 2
	errChan := make(chan error, numGoroutines)

	go fromHost(logger, session, channel, errChan)
	go fromGuest(logger, session, channel, errChan)

	return <-errChan
}

// fromHost processes terminal output from the Host.
func fromHost(
	logger *zap.Logger,
	session *session.Session,
	channel api.GuestService_TerminalChannelServer,
	errChan chan error,
) {
	for {
		select {
		case chunk := <-session.TerminalOutputChan:
			if err := channel.Send(&api.GuestTerminalResponse{
				Operation: &api.GuestTerminalResponse_Output{
					Output: &api.Data{
						Data: chunk,
					},
				},
			}); err != nil {
				logger.Warn("failed to send the host's terminal output to the guest", zap.Error(err))
				errChan <- err
				return
			}
		case <-channel.Context().Done():
			logger.Warn("channel was closed by the guest", zap.Error(channel.Context().Err()))
			errChan <- nil
			return
		case <-session.Context().Done():
			logger.Warn("lost connection with the terminal host")
			errChan <- status.Errorf(codes.Aborted, "lost connection with the terminal host")
			return
		}
	}
}

// fromGuest processes terminal input and other commands from the Guest.
func fromGuest(
	logger *zap.Logger,
	session *session.Session,
	channel api.GuestService_TerminalChannelServer,
	errChan chan error,
) {
	for {
		requestFromGuest, err := channel.Recv()
		if err != nil {
			logger.Warn("failed to receive terminal input/commands from the guest", zap.Error(err))
			errChan <- err
			return
		}

		switch msg := requestFromGuest.Operation.(type) {
		case *api.GuestTerminalRequest_ChangeDimensions:
			select {
			case session.ChangeDimensionsChan <- msg.ChangeDimensions:
				continue
			case <-channel.Context().Done():
				logger.Warn("channel was closed by the guest", zap.Error(channel.Context().Err()))
				errChan <- nil
				return
			case <-session.Context().Done():
				logger.Warn("lost connection with the terminal host")
				errChan <- status.Errorf(codes.Aborted, "lost connection with the terminal host")
				return
			}
		case *api.GuestTerminalRequest_Input:
			select {
			case session.TerminalInputChan <- msg.Input.Data:
				continue
			case <-channel.Context().Done():
				logger.Warn("channel was closed by the guest", zap.Error(channel.Context().Err()))
				errChan <- nil
				return
			case <-session.Context().Done():
				logger.Warn("lost connection with the terminal host")
				errChan <- status.Errorf(codes.Aborted, "lost connection with the terminal host")
				return
			}
		default:
			logger.Warn("expected a TerminalDimensions or a Data message, got something else")
			errChan <- status.Errorf(codes.FailedPrecondition, "expected a TerminalDimensions or a Data message")
			return
		}
	}
}

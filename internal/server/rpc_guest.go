package server

import (
	"github.com/cirruslabs/terminal/internal/api"
	"github.com/cirruslabs/terminal/internal/server/session"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (ts *TerminalServer) TerminalChannel(channel api.GuestService_TerminalChannelServer) error {
	// Guest begins the session by sending a Hello message
	// with the credentials of the terminal it wants to talk to
	requestFromGuest, err := channel.Recv()
	if err != nil {
		return err
	}
	helloFromGuest := requestFromGuest.GetHello()
	if helloFromGuest == nil {
		return status.Errorf(codes.FailedPrecondition, "expected a Hello message")
	}

	// Find a terminal with the requested locator
	terminal := ts.findTerminal(helloFromGuest.Locator)
	if terminal == nil {
		return status.Errorf(codes.NotFound, "terminal with locator %q is not registered on this server",
			helloFromGuest.Locator)
	}

	// Authenticate the Guest
	if !terminal.IsSecretValid(helloFromGuest.Secret) {
		return status.Errorf(codes.PermissionDenied, "invalid secret")
	}

	// Start a new session on this terminal
	session := session.New(channel.Context(), helloFromGuest.RequestedDimensions)
	defer session.Close()

	if err := terminal.RegisterSession(session); err != nil {
		return err
	}
	defer terminal.UnregisterSession(session)

	// Broadcast the created session
	select {
	case terminal.NewSessionChan <- session:
		// OK, proceed with session I/O below
	case <-channel.Context().Done():
		// Connection with the Guest was terminated before the Host had a chance to pick up our session
		return nil
	}

	// A way to terminate channel if we receive at least one error from one of the two Goroutines below
	const numGoroutines = 2
	errChan := make(chan error, numGoroutines)

	// Process terminal output from the Host
	go func() {
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
					errChan <- err
					return
				}
			case <-channel.Context().Done():
				errChan <- nil
				return
			case <-session.Context().Done():
				errChan <- status.Errorf(codes.Aborted, "lost connection with the terminal host")
				return
			}
		}
	}()

	go func() {
		// Process terminal input and other commands from the Guest
		for {
			requestFromGuest, err = channel.Recv()
			if err != nil {
				errChan <- err
				return
			}

			switch msg := requestFromGuest.Operation.(type) {
			case *api.GuestTerminalRequest_ChangeDimensions:
				select {
				case session.ChangeDimensionsChan <- msg.ChangeDimensions:
					continue
				case <-channel.Context().Done():
					errChan <- nil
					return
				case <-session.Context().Done():
					errChan <- status.Errorf(codes.Aborted, "lost connection with the terminal host")
					return
				}
			case *api.GuestTerminalRequest_Input:
				select {
				case session.TerminalInputChan <- msg.Input.Data:
					continue
				case <-channel.Context().Done():
					errChan <- nil
					return
				case <-session.Context().Done():
					errChan <- status.Errorf(codes.Aborted, "lost connection with the terminal host")
					return
				}
			default:
				errChan <- status.Errorf(codes.FailedPrecondition, "expected a TerminalDimensions or a Data message")
				return
			}
		}
	}()

	return <-errChan
}

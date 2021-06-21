package server

import (
	"github.com/cirruslabs/terminal/internal/api"
	"github.com/cirruslabs/terminal/internal/server/terminal"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (ts *TerminalServer) ControlChannel(channel api.HostService_ControlChannelServer) error {
	// Host begins with sending a Hello message that contains the credentials it trusts
	requestFromHost, err := channel.Recv()
	if err != nil {
		return err
	}
	helloFromHost := requestFromHost.GetHello()
	if helloFromHost == nil {
		return status.Errorf(codes.FailedPrecondition, "expected a Hello message")
	}

	// Create and register a new terminal associated with this Host
	terminal := terminal.New(ts.generateLocator(), terminal.WithTrustedSecret(helloFromHost.TrustedSecret))
	defer terminal.Close()

	if err := ts.registerTerminal(terminal); err != nil {
		return err
	}
	defer ts.unregisterTerminal(terminal)

	// Tell the Host it's locator
	if err := channel.Send(&api.HostControlResponse{
		Operation: &api.HostControlResponse_Hello_{
			Hello: &api.HostControlResponse_Hello{
				Locator: terminal.Locator(),
			},
		},
	}); err != nil {
		return err
	}

	for {
		select {
		case session := <-terminal.NewSessionChan:
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
				return err
			}
		case <-channel.Context().Done():
			// The Guest has left and there's nothing we can do about it except close and unregister it's terminal
			return channel.Context().Err()
		}
	}
}

func (ts *TerminalServer) DataChannel(channel api.HostService_DataChannelServer) error {
	// Host begins the channel by sending a Hello message
	// with the token it received from the control channel
	requestFromHost, err := channel.Recv()
	if err != nil {
		return err
	}
	helloFromHost := requestFromHost.GetHello()
	if helloFromHost == nil {
		return status.Errorf(codes.FailedPrecondition, "expected a Hello message")
	}

	terminal := ts.findTerminal(helloFromHost.Locator)
	if terminal == nil {
		return status.Errorf(codes.NotFound, "terminal with locator %q not found", helloFromHost.Locator)
	}

	session := terminal.FindSession(helloFromHost.Token)
	if session == nil {
		return status.Errorf(codes.NotFound, "terminal %q has no active session with the specified token",
			helloFromHost.Token)
	}

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
			case <-session.Context().Done():
				errChan <- session.Context().Err()
				return
			case <-channel.Context().Done():
				errChan <- channel.Context().Err()
				return
			}

			if err := channel.Send(responseToHost); err != nil {
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
				errChan <- err
				return
			}
			outputFromHost := requestFromHost.GetOutput()
			if outputFromHost == nil {
				errChan <- status.Errorf(codes.FailedPrecondition, "expected a Data message")
				return
			}

			select {
			case session.TerminalOutputChan <- outputFromHost.Data:
				continue
			case <-session.Context().Done():
				errChan <- session.Context().Err()
				return
			case <-channel.Context().Done():
				errChan <- channel.Context().Err()
				return
			}
		}
	}()

	return <-errChan
}

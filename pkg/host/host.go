// +build !windows

package host

import (
	"context"
	"errors"
	"fmt"
	"github.com/cirruslabs/terminal/internal/api"
	"github.com/cirruslabs/terminal/pkg/host/session"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"io"
	"sync"
	"time"
)

const (
	defaultServerAddress = "terminal.cirrus-ci.com"
)

var (
	ErrProtocol = errors.New("protocol error")
	ErrSecurity = errors.New("security violation")
)

func New(opts ...Option) (*TerminalHost, error) {
	client := &TerminalHost{
		sessions: make(map[string]*session.Session),
	}

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
		if err := th.locatorCallback(helloFromServer.Locator); err != nil {
			return err
		}
	}

	var sessionWG sync.WaitGroup
	defer sessionWG.Wait()

	// Loop waiting for the data channels to be requested
	for {
		controlFromServer, err = controlChannel.Recv()
		if err != nil {
			select {
			// A special case here is needed to prevent
			// us from returning the gRPC's Status struct,
			// because context.Cancelled would be more
			// appropriate, e.g. to check for the exact
			// error in tests
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

		session := session.New(th.logger, dataChannelRequest.Token)
		sessionWG.Add(1)

		go func() {
			th.registerSession(session)
			session.Run(ctx, hostService, helloFromServer.Locator, dataChannelRequest.RequestedDimensions)
			th.unregisterSession(session)
			sessionWG.Done()
		}()
	}
}

func (th *TerminalHost) LastActivity() time.Time {
	th.sessionsLock.Lock()
	defer th.sessionsLock.Unlock()

	var result time.Time

	for _, session := range th.sessions {
		sessionLastActivity := session.LastActivity()
		if sessionLastActivity.After(result) {
			result = sessionLastActivity
		}
	}

	return result
}

func (th *TerminalHost) NumSessions() int {
	th.sessionsLock.Lock()
	defer th.sessionsLock.Unlock()

	return len(th.sessions)
}

func (th *TerminalHost) NumSessionsFunc(f func(session *session.Session) bool) int {
	th.sessionsLock.Lock()
	defer th.sessionsLock.Unlock()

	var result int

	for _, session := range th.sessions {
		if f(session) {
			result++
		}
	}

	return result
}

func (th *TerminalHost) registerSession(session *session.Session) {
	th.sessionsLock.Lock()
	defer th.sessionsLock.Unlock()

	th.sessions[session.Token()] = session
}

func (th *TerminalHost) unregisterSession(session *session.Session) {
	th.sessionsLock.Lock()
	defer th.sessionsLock.Unlock()

	delete(th.sessions, session.Token())
}

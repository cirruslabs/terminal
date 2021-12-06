//go:build !windows
// +build !windows

package host

import (
	"context"
	"errors"
	"fmt"
	"github.com/cirruslabs/cirrus-ci-agent/pkg/grpchelper"
	"github.com/cirruslabs/terminal/internal/api"
	"github.com/cirruslabs/terminal/pkg/host/session"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"sync"
	"time"
)

const (
	defaultServerAddress = "https://terminal.cirrus-ci.com:443"
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
		client.logger = zap.NewNop()
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
	target, transportSecurity := grpchelper.TransportSettingsAsDialOption(th.serverAddress)

	clientConn, err := grpc.Dial(target, transportSecurity)
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

	th.lastConnectionMtx.Lock()
	th.lastConnection = time.Now()
	th.lastConnectionMtx.Unlock()

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
			case <-ctx.Done():
				return ctx.Err()
			default:
				return err
			}
		}
		dataChannelRequest := controlFromServer.GetDataChannelRequest()
		if dataChannelRequest == nil {
			return fmt.Errorf("%w: should've received a DataChannelRequest message", ErrProtocol)
		}

		session := session.New(th.logger, dataChannelRequest.Token, th.shellEnv)
		sessionWG.Add(1)

		go func() {
			th.registerSession(session)
			session.Run(ctx, hostService, helloFromServer.Locator, dataChannelRequest.RequestedDimensions)
			th.unregisterSession(session)
			sessionWG.Done()
		}()
	}
}

func (th *TerminalHost) LastConnection() time.Time {
	th.sessionsLock.Lock()
	defer th.sessionsLock.Unlock()

	return th.lastConnection
}

func (th *TerminalHost) LastRegistration() time.Time {
	th.sessionsLock.Lock()
	defer th.sessionsLock.Unlock()

	return th.lastRegistration
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

	// Perhaps there was an intermittent session that was just closed
	// but generated an activity that is way more recent than any of
	// the active sessions
	if th.lastActivity.After(result) {
		return th.lastActivity
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

	now := time.Now()
	if now.After(th.lastRegistration) {
		th.lastRegistration = now
	}

	th.sessions[session.Token()] = session
}

func (th *TerminalHost) unregisterSession(session *session.Session) {
	th.sessionsLock.Lock()
	defer th.sessionsLock.Unlock()

	// Keep track of last activity time for intermittent sessions
	lastActivity := session.LastActivity()
	if lastActivity.After(th.lastActivity) {
		th.lastActivity = lastActivity
	}

	delete(th.sessions, session.Token())
}

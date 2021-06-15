package server

import (
	"context"
	"errors"
	"github.com/cirruslabs/terminal/internal/api"
	"github.com/cirruslabs/terminal/internal/server/terminal"
	"github.com/google/uuid"
	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"io"
	"net"
	"net/http"
	"sync"
)

type TerminalServer struct {
	logger *logrus.Logger

	terminalsLock sync.RWMutex
	terminals     map[string]*terminal.Terminal

	guestAddress  string
	guestListener net.Listener
	api.UnimplementedGuestServiceServer

	hostAddress  string
	hostListener net.Listener
	api.UnimplementedHostServiceServer

	websocketOriginFunc WebsocketOriginFunc
	generateLocator     LocatorGenerator
}

func New(opts ...Option) (*TerminalServer, error) {
	ts := &TerminalServer{
		terminals: make(map[string]*terminal.Terminal),
	}

	// Apply options
	for _, opt := range opts {
		opt(ts)
	}

	// Apply defaults
	if ts.logger == nil {
		ts.logger = logrus.New()
		ts.logger.Out = io.Discard
	}
	if ts.websocketOriginFunc == nil {
		ts.websocketOriginFunc = func(*http.Request) bool {
			return false
		}
	}
	if ts.generateLocator == nil {
		ts.generateLocator = func() string {
			return uuid.New().String()
		}
	}
	if ts.guestAddress == "" {
		ts.guestAddress = "0.0.0.0:8080"
	}
	if ts.hostAddress == "" {
		ts.hostAddress = "0.0.0.0:8081"
	}

	var err error

	ts.guestListener, err = net.Listen("tcp", ts.guestAddress)
	if err != nil {
		return nil, err
	}
	ts.hostListener, err = net.Listen("tcp", ts.hostAddress)
	if err != nil {
		return nil, err
	}

	return ts, nil
}

func (ts *TerminalServer) Run(ctx context.Context) error {
	// Create a sub-context to let the first failing Goroutine to start the cancellation process
	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// gRPC server that deals with Hosts
	hostServer := grpc.NewServer()
	api.RegisterHostServiceServer(hostServer, ts)

	go func() {
		defer cancel()

		ts.logger.Infof("starting HostService gRPC server at %s", ts.hostListener.Addr().String())

		if err := hostServer.Serve(ts.hostListener); err != nil {
			if !errors.Is(err, grpc.ErrServerStopped) {
				ts.logger.Warnf("HostService gRPC server failed: %v", err)
			}
		}
	}()

	// gRPC-Web server that deals with Guests
	guestServer := grpc.NewServer()
	api.RegisterGuestServiceServer(guestServer, ts)

	// Since we use gRPC-Web with Websockets transport, we need additional wrapping
	wrappedGuestServer := grpcweb.WrapServer(
		guestServer,
		grpcweb.WithWebsockets(true),
		grpcweb.WithWebsocketOriginFunc(ts.websocketOriginFunc),
	)

	guestHTTPServer := http.Server{
		Handler: wrappedGuestServer,
	}

	go func() {
		defer cancel()

		ts.logger.Infof("starting GuestService gRPC-Web server at %s", ts.guestListener.Addr().String())

		if err := guestHTTPServer.Serve(ts.guestListener); err != nil {
			if !errors.Is(err, http.ErrServerClosed) {
				ts.logger.Warnf("GuestService gRPC-Web server failed: %v", err)
			}
		}
	}()

	<-subCtx.Done()

	hostServer.GracefulStop()
	if err := guestHTTPServer.Shutdown(context.Background()); err != nil {
		return err
	}

	return nil
}

func (ts *TerminalServer) GuestServerAddress() string {
	return ts.guestListener.Addr().String()
}

func (ts *TerminalServer) HostServerAddress() string {
	return ts.hostListener.Addr().String()
}

func (ts *TerminalServer) RegisterTerminal(terminal *terminal.Terminal) {
	ts.terminalsLock.Lock()
	defer ts.terminalsLock.Unlock()

	ts.terminals[terminal.Locator()] = terminal
}

func (ts *TerminalServer) FindTerminal(locator string) *terminal.Terminal {
	ts.terminalsLock.RLock()
	defer ts.terminalsLock.RUnlock()

	terminal, ok := ts.terminals[locator]
	if !ok {
		return nil
	}

	return terminal
}

func (ts *TerminalServer) UnregisterTerminal(terminal *terminal.Terminal) {
	ts.terminalsLock.Lock()
	defer ts.terminalsLock.Unlock()

	delete(ts.terminals, terminal.Locator())
}
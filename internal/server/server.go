package server

import (
	"context"
	"errors"
	"fmt"
	"github.com/cirruslabs/terminal/internal/api"
	"github.com/cirruslabs/terminal/internal/server/terminal"
	"github.com/google/uuid"
	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/sirupsen/logrus"
	"github.com/soheilhy/cmux"
	"google.golang.org/grpc"
	"io"
	"net"
	"net/http"
	"sync"
)

var ErrNewTerminalRefused = errors.New("refusing to register new terminal")

type TerminalServer struct {
	logger *logrus.Logger

	terminalsLock sync.RWMutex
	terminals     map[string]*terminal.Terminal

	address  string
	listener net.Listener

	guestUsesNoGRPCWebWrapping bool

	api.UnimplementedGuestServiceServer
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
	if ts.address == "" {
		ts.address = "0.0.0.0:0"
	}

	var err error

	ts.listener, err = net.Listen("tcp", ts.address)
	if err != nil {
		return nil, err
	}

	return ts, nil
}

func (ts *TerminalServer) Run(ctx context.Context) (err error) {
	// Create a sub-context to let the first failing Goroutine to start the cancellation process
	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	mux := cmux.New(ts.listener)

	// gRPC server that deals with Hosts
	hostServer := grpc.NewServer()
	api.RegisterHostServiceServer(hostServer, ts)

	// nolint:nestif // moving these into separate functions would make the whole thing even more complicated
	if ts.guestUsesNoGRPCWebWrapping {
		api.RegisterGuestServiceServer(hostServer, ts)
	} else {
		guestListener := mux.Match(
			cmux.HTTP1HeaderField("content-type", "application/grpc-web+proto"),
			cmux.HTTP1HeaderField("content-type", "application/grpc-web+proto"),
			cmux.HTTP1HeaderField("content-type", "application/grpc-web-text"),
			cmux.HTTP1HeaderField("content-type", "application/grpc-web-text"),
		)
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

			ts.logger.Infof("starting GuestService gRPC-Web server at %s", guestListener.Addr().String())

			if err := guestHTTPServer.Serve(guestListener); err != nil {
				if !errors.Is(err, http.ErrServerClosed) {
					ts.logger.Warnf("GuestService gRPC-Web server failed: %v", err)
				}
			}
		}()

		defer func() {
			if localErr := guestHTTPServer.Shutdown(context.Background()); localErr != nil {
				err = localErr
			}
		}()
	}

	hostListener := mux.MatchWithWriters(cmux.HTTP2MatchHeaderFieldSendSettings("content-type", "application/grpc"))

	go func() {
		defer cancel()

		ts.logger.Infof("starting HostService gRPC server at %s", hostListener.Addr().String())

		if err := hostServer.Serve(hostListener); err != nil {
			if !errors.Is(err, grpc.ErrServerStopped) {
				ts.logger.Warnf("HostService gRPC server failed: %v", err)
			}
		}
	}()
	defer hostServer.GracefulStop()

	go func() {
		defer cancel()

		if err := mux.Serve(); err != nil {
			ts.logger.Warnf("GuestService gRPC-Web server failed: %v", err)
		}
	}()

	<-subCtx.Done()

	return nil
}

func (ts *TerminalServer) ServerAddress() string {
	return ts.listener.Addr().String()
}

func (ts *TerminalServer) registerTerminal(terminal *terminal.Terminal) error {
	ts.terminalsLock.Lock()
	defer ts.terminalsLock.Unlock()

	if _, ok := ts.terminals[terminal.Locator()]; ok {
		return fmt.Errorf("%w: a terminal with the same locator already exists", ErrNewTerminalRefused)
	}

	ts.terminals[terminal.Locator()] = terminal

	return nil
}

func (ts *TerminalServer) findTerminal(locator string) *terminal.Terminal {
	ts.terminalsLock.RLock()
	defer ts.terminalsLock.RUnlock()

	return ts.terminals[locator]
}

func (ts *TerminalServer) unregisterTerminal(terminal *terminal.Terminal) {
	ts.terminalsLock.Lock()
	defer ts.terminalsLock.Unlock()

	delete(ts.terminals, terminal.Locator())
}

package server

import (
	"context"
	"crypto/tls"
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

	address   string
	listener  net.Listener
	tlsConfig *tls.Config

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

	if ts.tlsConfig == nil {
		ts.listener, err = net.Listen("tcp", ts.address)
		if err != nil {
			return nil, err
		}
	} else {
		ts.listener, err = tls.Listen("tcp", ts.address, ts.tlsConfig)
		if err != nil {
			return nil, err
		}
	}

	return ts, nil
}

func (ts *TerminalServer) Run(ctx context.Context) (err error) {
	// Create a sub-context to let the first failing Goroutine to start the cancellation process
	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	mux := cmux.New(ts.listener)
	defer mux.Close()

	grpcServer := grpc.NewServer()
	defer grpcServer.GracefulStop()
	api.RegisterHostServiceServer(grpcServer, ts)
	api.RegisterGuestServiceServer(grpcServer, ts)

	// Since we use gRPC-Web with Websockets transport, we need additional wrapping
	webSocketServer := http.Server{
		Handler: grpcweb.WrapServer(
			grpcServer,
			grpcweb.WithWebsockets(true),
			grpcweb.WithWebsocketOriginFunc(ts.websocketOriginFunc),
		),
	}
	defer func() {
		if localErr := webSocketServer.Shutdown(context.Background()); localErr != nil {
			err = localErr
		}
	}()

	webSocketListener := mux.Match(
		cmux.HTTP1HeaderField("content-type", "application/grpc-web+proto"),
		cmux.HTTP1HeaderField("content-type", "application/grpc-web+proto"),
		cmux.HTTP1HeaderField("content-type", "application/grpc-web-text"),
		cmux.HTTP1HeaderField("content-type", "application/grpc-web-text"),
		cmux.HTTP1HeaderField("Sec-WebSocket-Protocol", "grpc-websockets"),
	)

	go func() {
		defer cancel()

		ts.logger.Infof("starting GuestService gRPC-Web server at %s", webSocketListener.Addr().String())

		if err := webSocketServer.Serve(webSocketListener); err != nil {
			if !errors.Is(err, http.ErrServerClosed) {
				ts.logger.Warnf("GuestService gRPC-Web server failed: %v", err)
			}
		}
	}()

	grpcListener := mux.MatchWithWriters(cmux.HTTP2MatchHeaderFieldSendSettings("content-type", "application/grpc"))

	go func() {
		defer cancel()

		ts.logger.Infof("starting HostService gRPC server at %s", grpcListener.Addr().String())

		if err := grpcServer.Serve(grpcListener); err != nil {
			if !errors.Is(err, grpc.ErrServerStopped) {
				ts.logger.Warnf("HostService gRPC server failed: %v", err)
			}
		}
	}()

	defaultListener := mux.Match(cmux.Any())
	go func() {
		defer cancel()
		if err := http.Serve(defaultListener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, "Please use gRPC over HTTP/2 or gRPC-web over HTTP/1")
		})); err != nil {
			ts.logger.Warnf("Default server failed: %v", err)
		}
	}()

	go func() {
		defer cancel()

		if err := mux.Serve(); err != nil {
			ts.logger.Warnf("mux server failed: %v", err)
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

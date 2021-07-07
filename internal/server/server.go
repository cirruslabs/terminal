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
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
	"io"
	"net"
	"net/http"
	"strings"
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

	generateLocator LocatorGenerator
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

	grpcServer := grpc.NewServer()
	defer grpcServer.Stop()
	api.RegisterHostServiceServer(grpcServer, ts)
	api.RegisterGuestServiceServer(grpcServer, ts)

	grpcWebServer := grpcweb.WrapServer(
		grpcServer,
		grpcweb.WithWebsockets(true),
		grpcweb.WithWebsocketOriginFunc(func(request *http.Request) bool {
			return true
		}),
	)

	go func() {
		defer cancel()

		server := http.Server{
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				contentType := r.Header.Get("content-type")
				switch {
				case grpcWebServer.IsGrpcWebSocketRequest(r):
					grpcWebServer.ServeHTTP(w, r)
				case strings.HasPrefix(contentType, "application/grpc-web"):
					grpcWebServer.ServeHTTP(w, r)
				case strings.HasPrefix(contentType, "application/grpc"):
					grpcServer.ServeHTTP(w, r)
				default:
					fmt.Fprint(w, "Please use gRPC over HTTP/2 or gRPC-web over HTTP/1")
				}
			}),
			TLSConfig: ts.tlsConfig,
		}

		var serveErr error
		if server.TLSConfig != nil {
			serveErr = server.ServeTLS(ts.listener, "", "")
		} else {
			// enable HTTP/2 without TLS aka h2c
			h2s := &http2.Server{}
			server.Handler = h2c.NewHandler(server.Handler, h2s)
			serveErr = server.Serve(ts.listener)
		}

		if serveErr != nil {
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

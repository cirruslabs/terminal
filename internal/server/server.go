package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/cirruslabs/terminal/internal/api"
	"github.com/cirruslabs/terminal/internal/server/terminal"
	"github.com/google/uuid"
	"github.com/gorilla/handlers"
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

	addresses []string
	listeners []net.Listener
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
	if len(ts.addresses) == 0 {
		ts.addresses = []string{"0.0.0.0:0"}
	}

	// Listen
	for _, address := range ts.addresses {
		listener, err := net.Listen("tcp", address)
		if err != nil {
			return nil, err
		}

		ts.listeners = append(ts.listeners, listener)
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

	grpcHandler := func(w http.ResponseWriter, r *http.Request) {
		contentType := r.Header.Get("content-type")
		switch {
		case strings.ToLower(r.Header.Get("Sec-Websocket-Protocol")) == "grpc-websockets":
			grpcWebServer.ServeHTTP(w, r)
		case strings.HasPrefix(contentType, "application/grpc-web"):
			grpcWebServer.ServeHTTP(w, r)
		case strings.HasPrefix(contentType, "application/grpc"):
			grpcServer.ServeHTTP(w, r)
		default:
			fmt.Fprint(w, "Please use gRPC over HTTP/2 or gRPC-web over HTTP/1")
		}
	}
	handlerWithLogging := handlers.CustomLoggingHandler(
		ts.logger.Writer(),
		http.HandlerFunc(grpcHandler),
		func(w io.Writer, p handlers.LogFormatterParams) {
			req := p.Request

			uri := req.RequestURI
			if uri == "" {
				uri = p.URL.RequestURI()
			}

			_, _ = fmt.Fprintln(w, req.Method, uri, req.Proto, p.StatusCode, p.Size)
		},
	)

	startServer := func(listener net.Listener) error {
		server := http.Server{
			Handler:   handlerWithLogging,
			TLSConfig: ts.tlsConfig,
		}

		ts.logger.Infof("Starting server on %s...", listener.Addr().String())

		if server.TLSConfig != nil {
			return server.ServeTLS(listener, "", "")
		}
		// enable HTTP/2 without TLS aka h2c
		h2s := &http2.Server{}
		server.Handler = h2c.NewHandler(server.Handler, h2s)
		return server.Serve(listener)
	}

	for _, listener := range ts.listeners {
		listener := listener
		go func() {
			defer cancel()

			if serverErr := startServer(listener); serverErr != nil {
				ts.logger.Warnf("server failed to start on %s: %v", listener.Addr().String(), err)
			}
		}()
	}

	<-subCtx.Done()

	return nil
}

func (ts *TerminalServer) Addresses() []string {
	var result []string

	for _, listener := range ts.listeners {
		result = append(result, listener.Addr().String())
	}

	return result
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

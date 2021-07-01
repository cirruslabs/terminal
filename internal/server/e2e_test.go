package server_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/cirruslabs/terminal/internal/api"
	"github.com/cirruslabs/terminal/internal/server"
	"github.com/cirruslabs/terminal/pkg/host"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"net/http"
	"strings"
	"testing"
)

func TestTerminalDimensionsCanBeChanged(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Initialize terminal server
	var serverOpts []server.Option

	logger := logrus.New()
	logger.SetLevel(logrus.TraceLevel)
	serverOpts = append(serverOpts, server.WithLogger(logger))

	serverOpts = append(serverOpts, server.WithWebsocketOriginFunc(func(request *http.Request) bool {
		return true
	}))

	serverOpts = append(serverOpts)

	terminalServer, err := server.New(serverOpts...)
	if err != nil {
		t.Fatal(err)
	}

	// Run terminal server
	terminalServerErrChan := make(chan error)
	go func() {
		terminalServerErrChan <- terminalServer.Run(ctx)
	}()

	// Initialize terminal host
	hostOpts := []host.Option{
		host.WithLogger(logger),
		host.WithServerAddress("http://" + terminalServer.ServerAddress()),
	}

	const secret = "fixed secret used in tests"
	hostOpts = append(hostOpts, host.WithTrustedSecret(secret))

	locatorChan := make(chan string)
	hostOpts = append(hostOpts, host.WithLocatorCallback(func(locator string) error {
		locatorChan <- locator
		return nil
	}))

	terminalHost, err := host.New(hostOpts...)
	if err != nil {
		t.Fatal(err)
	}

	// Run terminal host
	terminalHostErrChan := make(chan error)
	go func() {
		terminalHostErrChan <- terminalHost.Run(ctx)
	}()

	// Collect the locator assigned to the host
	locator := <-locatorChan

	// Emulate guest: open up a terminal channel, just like a web UI would do
	clientConn, err := grpc.Dial(terminalServer.ServerAddress(), grpc.WithInsecure())
	if err != nil {
		t.Fatal(err)
	}
	guestService := api.NewGuestServiceClient(clientConn)
	terminalChannel, err := guestService.TerminalChannel(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Initialize the channel, providing the credentials and the initial terminal size
	// (using arbitrary canary values with low probability of appearing in the terminal output)
	const (
		initialTerminalWidthColumns = 123
		initialTerminalHeightRows   = 456
	)

	if err := terminalChannel.Send(&api.GuestTerminalRequest{
		Operation: &api.GuestTerminalRequest_Hello_{
			Hello: &api.GuestTerminalRequest_Hello{
				Locator: locator,
				Secret:  secret,
				RequestedDimensions: &api.TerminalDimensions{
					WidthColumns: initialTerminalWidthColumns,
					HeightRows:   initialTerminalHeightRows,
				},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	// A couple of helper functions that will be re-used below
	queryTerminalSize := func() {
		if err := terminalChannel.Send(&api.GuestTerminalRequest{
			Operation: &api.GuestTerminalRequest_Input{
				Input: &api.Data{
					Data: []byte("echo -e \"cols\\nlines\" | tput -S\n"),
				},
			},
		}); err != nil {
			t.Fatal(err)
		}
	}
	waitForCanary := func(canary string) {
		buf := bytes.NewBuffer([]byte{})

		for {
			helloFromServer, err := terminalChannel.Recv()
			if err != nil {
				t.Fatal(err)
			}

			buf.Write(helloFromServer.GetOutput().Data)

			if strings.Contains(buf.String(), canary) {
				break
			}
		}
	}

	queryTerminalSize()
	waitForCanary(fmt.Sprintf("%d\r\n%d", initialTerminalWidthColumns, initialTerminalHeightRows))

	// Now change terminal size on-the-fly
	const (
		onTheFlyTerminalWidthColumns = 111
		onTheFlyTerminalHeightRows   = 222
	)

	if err := terminalChannel.Send(&api.GuestTerminalRequest{
		Operation: &api.GuestTerminalRequest_ChangeDimensions{
			ChangeDimensions: &api.TerminalDimensions{
				WidthColumns: onTheFlyTerminalWidthColumns,
				HeightRows:   onTheFlyTerminalHeightRows,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	queryTerminalSize()
	waitForCanary(fmt.Sprintf("%d\r\n%d", onTheFlyTerminalWidthColumns, onTheFlyTerminalHeightRows))

	assert.Equal(t, 1, terminalHost.NumSessions(), "terminal host should run exactly 1 session")

	cancel()

	if err := <-terminalHostErrChan; err != nil && !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}

	assert.Equal(t, 0, terminalHost.NumSessions(), "terminal host should not run any sessions")

	if err := <-terminalServerErrChan; err != nil && !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestWebsocketOriginChecking(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Initialize terminal server
	serverOpts := []server.Option{
		server.WithServerAddress("127.0.0.1:0"),
	}

	logger := logrus.New()
	logger.SetLevel(logrus.TraceLevel)
	serverOpts = append(serverOpts, server.WithLogger(logger))

	const goodOrigin = "https://example.com"
	serverOpts = append(serverOpts, server.WithWebsocketOriginFunc(func(request *http.Request) bool {
		return request.Header.Get("Origin") == goodOrigin
	}))

	terminalServer, err := server.New(serverOpts...)
	if err != nil {
		t.Fatal(err)
	}

	// Run terminal server
	terminalServerErrChan := make(chan error)
	go func() {
		terminalServerErrChan <- terminalServer.Run(ctx)
	}()

	// Craft WebSocket URL and headers that should be generally acceptable by our gRPC-Web instance,
	// excluding the Origin header
	webSocketURL := "ws://" + terminalServer.ServerAddress() + "/GuestService/TerminalChannel"
	baseHeaders := http.Header{}
	baseHeaders.Add("Sec-Websocket-Protocol", "grpc-websockets")

	// Set an acceptable Origin header and ensure that the connection is upgraded
	goodHeaders := baseHeaders.Clone()
	goodHeaders.Add("Origin", goodOrigin)
	goodHeaders.Add("Content-Type", "application/grpc-web-text")
	_, resp, err := websocket.DefaultDialer.Dial(webSocketURL, goodHeaders)
	require.NoError(t, err)
	require.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)
	_ = resp.Body.Close()

	// Set an unacceptable Origin header and ensure the request is denied
	badHeaders := baseHeaders.Clone()
	badHeaders.Add("Origin", "https://bad.origin")
	badHeaders.Add("Content-Type", "application/grpc-web-text")
	_, resp, err = websocket.DefaultDialer.Dial(webSocketURL, badHeaders)
	require.Equal(t, websocket.ErrBadHandshake, err)
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
	_ = resp.Body.Close()

	cancel()
	if err := <-terminalServerErrChan; err != nil && !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

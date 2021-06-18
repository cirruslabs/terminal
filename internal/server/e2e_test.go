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
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"net/http"
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

	// Disable gRPC-Web wrapping so that we can use a normal gRPC client that talks over HTTP/2 for this test
	serverOpts = append(serverOpts, server.WithGuestUsesNoGRPCWebWrapping())

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
		host.WithServerAddress(terminalServer.HostServerAddress()),
		host.WithServerInsecure(),
	}

	const secret = "fixed secret used in tests"
	hostOpts = append(hostOpts, host.WithTrustedSecret(secret))

	locatorChan := make(chan string)
	hostOpts = append(hostOpts, host.WithLocatorCallback(func(locator string) {
		locatorChan <- locator
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
	clientConn, err := grpc.Dial(terminalServer.GuestServerAddress(), grpc.WithInsecure())
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
		initialTerminalHeightRows
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
		for {
			helloFromServer, err := terminalChannel.Recv()
			if err != nil {
				t.Fatal(err)
			}

			if bytes.Contains(helloFromServer.GetOutput().Data, []byte(canary)) {
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

	cancel()
	if err := <-terminalServerErrChan; err != nil && !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := <-terminalHostErrChan; err != nil && !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestWebsocketOriginChecking(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Initialize terminal server
	serverOpts := []server.Option{
		server.WithGuestServerAddress("127.0.0.1:0"),
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
	webSocketURL := "ws://" + terminalServer.GuestServerAddress() + "/GuestService/TerminalChannel"
	baseHeaders := http.Header{}
	baseHeaders.Add("Sec-Websocket-Protocol", "grpc-websockets")

	// Set an acceptable Origin header and ensure that the connection is upgraded
	goodHeaders := baseHeaders.Clone()
	goodHeaders.Add("Origin", goodOrigin)
	_, resp, err := websocket.DefaultDialer.Dial(webSocketURL, goodHeaders)
	require.NoError(t, err)
	require.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)
	_ = resp.Body.Close()

	// Set an unacceptable Origin header and ensure the request is denied
	badHeaders := baseHeaders.Clone()
	badHeaders.Add("Origin", "https://bad.origin")
	_, resp, err = websocket.DefaultDialer.Dial(webSocketURL, badHeaders)
	require.Equal(t, websocket.ErrBadHandshake, err)
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
	_ = resp.Body.Close()

	cancel()
	if err := <-terminalServerErrChan; err != nil && !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

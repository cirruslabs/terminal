package server_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/cirruslabs/terminal/internal/api"
	"github.com/cirruslabs/terminal/internal/server"
	"github.com/cirruslabs/terminal/pkg/host"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"strings"
	"testing"
)

func TestTerminalDimensionsCanBeChanged(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	serverAddress := ":11111"

	// Initialize terminal server
	var serverOpts []server.Option

	logger := logrus.New()
	logger.SetLevel(logrus.TraceLevel)
	serverOpts = append(serverOpts, server.WithLogger(logger))
	serverOpts = append(serverOpts, server.WithAddresses([]string{serverAddress}))

	terminalServer, err := server.New(serverOpts...)
	if err != nil {
		t.Fatal(err)
	}

	// Run terminal server
	terminalServerErrChan := make(chan error)
	go func() {
		serverError := terminalServer.Run(ctx)
		terminalServerErrChan <- serverError
	}()

	// Initialize terminal host
	hostOpts := []host.Option{
		host.WithLogger(logger),
		host.WithServerAddress("http://" + serverAddress),
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
	var locator string
	select {
	case locator = <-locatorChan:
	case err := <-terminalHostErrChan:
		t.Fatal(err)
	}

	// Emulate guest: open up a terminal channel, just like a web UI would do
	clientConn, err := grpc.Dial(serverAddress, grpc.WithInsecure())
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

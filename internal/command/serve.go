package command

import (
	"fmt"
	"github.com/cirruslabs/terminal/internal/server"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"net/http"
	"strings"
)

var logLevel string
var guestServerAddress, hostServerAddress string
var allowedOrigins []string

func serve(cmd *cobra.Command, args []string) error {
	logLevel, err := logrus.ParseLevel(logLevel)
	if err != nil {
		return err
	}
	logger := logrus.New()
	logger.SetLevel(logLevel)

	websocketOriginFunc := func(request *http.Request) bool {
		for _, allowedOrigin := range allowedOrigins {
			if request.Header.Get("Origin") == allowedOrigin {
				return true
			}
		}

		return false
	}

	terminalServer, err := server.New(
		server.WithLogger(logger),
		server.WithGuestServerAddress(guestServerAddress),
		server.WithHostServerAddress(hostServerAddress),
		server.WithWebsocketOriginFunc(websocketOriginFunc),
	)
	if err != nil {
		return err
	}

	return terminalServer.Run(cmd.Context())
}

func newServeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve [flags]",
		Short: "Run terminal server with guest (gRPC-Web over WebSocket) and host (gRPC) servers",
		RunE:  serve,
	}

	var logLevelNames []string
	for _, level := range logrus.AllLevels {
		logLevelNames = append(logLevelNames, level.String())
	}
	cmd.PersistentFlags().StringVar(&logLevel, "log-level", "info",
		fmt.Sprintf("logging level (possible levels: %s)", strings.Join(logLevelNames, ", ")))

	cmd.PersistentFlags().StringVar(&guestServerAddress, "guest-server-address", "0.0.0.0:8080", "address for the guest server to listen on")
	cmd.PersistentFlags().StringVar(&hostServerAddress, "host-server-address", "0.0.0.0:8081", "address for the host server to listen on")

	cmd.PersistentFlags().StringSliceVar(&allowedOrigins, "allowed-origins", []string{},
		"a list comma-separated origins that are allowed to talk with the guest's gRPC-Web WebSocket server")

	return cmd
}

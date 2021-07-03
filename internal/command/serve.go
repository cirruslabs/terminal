package command

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/cirruslabs/terminal/internal/server"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"math/big"
	"net/http"
	"os"
	"strings"
)

var logLevel string
var serverAddress string
var tlsEphemeral bool
var tlsCertFile, tlsKeyFile string
var allowedOrigins []string

func runServe(cmd *cobra.Command, args []string) error {
	logLevel, err := logrus.ParseLevel(logLevel)
	if err != nil {
		return err
	}
	logger := logrus.New()
	logger.SetLevel(logLevel)

	var tlsConfig *tls.Config

	if tlsCertFile != "" || tlsKeyFile != "" {
		certificate, err := tls.LoadX509KeyPair(tlsCertFile, tlsKeyFile)
		if err != nil {
			return err
		}

		tlsConfig = &tls.Config{
			Certificates: []tls.Certificate{certificate},
			MinVersion:   tls.VersionTLS13,
		}
	} else if tlsEphemeral {
		privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return err
		}

		cert := &x509.Certificate{
			SerialNumber: big.NewInt(1),
		}

		certBytes, err := x509.CreateCertificate(rand.Reader, cert, cert, privateKey.Public(), privateKey)
		if err != nil {
			return err
		}

		tlsConfig = &tls.Config{
			Certificates: []tls.Certificate{
				{
					Certificate: [][]byte{certBytes},
					PrivateKey:  privateKey,
				},
			},
			MinVersion: tls.VersionTLS13,
		}
	}

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
		server.WithServerAddress(serverAddress),
		server.WithWebsocketOriginFunc(websocketOriginFunc),
		server.WithTLSConfig(tlsConfig),
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
		RunE:  runServe,
	}

	var logLevelNames []string
	for _, level := range logrus.AllLevels {
		logLevelNames = append(logLevelNames, level.String())
	}
	cmd.PersistentFlags().StringVar(&logLevel, "log-level", "info",
		fmt.Sprintf("logging level (possible levels: %s)", strings.Join(logLevelNames, ", ")))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	cmd.PersistentFlags().StringVarP(&serverAddress, "listen", "l", fmt.Sprintf(":%s", port),
		"address to listen on")

	cmd.PersistentFlags().BoolVar(&tlsEphemeral, "tls-ephemeral", false,
		"enable TLS and generate a self-signed and ephemeral certificate and key")
	cmd.PersistentFlags().StringVar(&tlsCertFile, "tls-cert-file", "",
		"enable TLS and use the specified certificate file (must also specify --tls-key-file)")
	cmd.PersistentFlags().StringVar(&tlsKeyFile, "tls-key-file", "",
		"enable TLS and use the specified key file (must also specify --tls-cert-file)")

	cmd.PersistentFlags().StringSliceVar(&allowedOrigins, "allowed-origins", []string{},
		"a list comma-separated origins that are allowed to talk with the guest's gRPC-Web WebSocket server")

	return cmd
}

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
	"os"
	"strings"
)

var logLevel string
var serverAddresses []string
var tlsEphemeral bool
var tlsCertFile, tlsKeyFile string

func runServe(cmd *cobra.Command, args []string) error {
	logLevel, err := logrus.ParseLevel(logLevel)
	if err != nil {
		return err
	}
	logger := logrus.New()
	logger.SetLevel(logLevel)
	logger.SetFormatter(&logrus.TextFormatter{
		DisableTimestamp: true,
	})

	var tlsConfig *tls.Config

	if tlsCertFile != "" || tlsKeyFile != "" {
		certificate, err := tls.LoadX509KeyPair(tlsCertFile, tlsKeyFile)
		if err != nil {
			return err
		}

		tlsConfig = &tls.Config{
			Certificates: []tls.Certificate{certificate},
			MinVersion:   tls.VersionTLS12,
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
			MinVersion: tls.VersionTLS12,
		}
	}

	terminalServer, err := server.New(
		server.WithLogger(logger),
		server.WithAddresses(serverAddresses),
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

	// nolint:ifshort // false-positive similar to https://github.com/esimonov/ifshort/issues/12
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	cmd.PersistentFlags().StringSliceVarP(&serverAddresses, "listen", "l", []string{fmt.Sprintf(":%s", port)},
		"addresses to listen on")

	cmd.PersistentFlags().BoolVar(&tlsEphemeral, "tls-ephemeral", false,
		"enable TLS and generate a self-signed and ephemeral certificate and key")
	cmd.PersistentFlags().StringVar(&tlsCertFile, "tls-cert-file", "",
		"enable TLS and use the specified certificate file (must also specify --tls-key-file)")
	cmd.PersistentFlags().StringVar(&tlsKeyFile, "tls-key-file", "",
		"enable TLS and use the specified key file (must also specify --tls-cert-file)")

	return cmd
}

package command

import (
	"cloud.google.com/go/compute/metadata"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/blendle/zapdriver"
	"github.com/cirruslabs/terminal/internal/server"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"math/big"
	"os"
)

var debug bool
var serverAddresses []string
var tlsEphemeral bool
var tlsCertFile, tlsKeyFile string

func getLogger() (*zap.Logger, error) {
	if debug {
		return zap.NewDevelopment()
	}

	return zap.NewProduction()
}

func runServe(cmd *cobra.Command, args []string) (err error) {
	var opts []server.Option

	// Initialize logger
	logger, err := getLogger()

	projectID, err := metadata.ProjectID()
	if err == nil {
		opts = append(opts, server.WithGCPProjectID(projectID))

		if debug {
			logger, err = zapdriver.NewDevelopment()
		}

		logger, err = zapdriver.NewProduction()
	}

	if err != nil {
		return err
	}
	defer func() {
		_ = logger.Sync()
	}()

	opts = append(opts, server.WithLogger(logger))

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

	opts = append(opts, server.WithTLSConfig(tlsConfig), server.WithAddresses(serverAddresses))

	terminalServer, err := server.New(opts...)
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

	cmd.PersistentFlags().BoolVar(&debug, "debug", false, "enable debugging")

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

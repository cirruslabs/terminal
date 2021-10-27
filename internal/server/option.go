package server

import (
	"crypto/tls"
	"go.uber.org/zap"
)

type Option func(*TerminalServer)

type LocatorGenerator func() string

func WithLogger(logger *zap.Logger) Option {
	return func(ts *TerminalServer) {
		ts.logger = logger
	}
}

func WithAddresses(addresses []string) Option {
	return func(ts *TerminalServer) {
		ts.addresses = addresses
	}
}

func WithLocatorGenerator(locatorGenerator LocatorGenerator) Option {
	return func(ts *TerminalServer) {
		ts.generateLocator = locatorGenerator
	}
}

func WithTLSConfig(tlsConfig *tls.Config) Option {
	return func(ts *TerminalServer) {
		ts.tlsConfig = tlsConfig
	}
}

func WithGCPProjectID(gcpProjectID string) Option {
	return func(ts *TerminalServer) {
		ts.gcpProjectID = gcpProjectID
	}
}

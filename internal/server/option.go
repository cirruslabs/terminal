package server

import (
	"crypto/tls"
	"github.com/sirupsen/logrus"
)

type Option func(*TerminalServer)

type LocatorGenerator func() string

func WithLogger(logger *logrus.Logger) Option {
	return func(ts *TerminalServer) {
		ts.logger = logger
	}
}

func WithServerAddress(address string) Option {
	return func(ts *TerminalServer) {
		ts.address = address
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

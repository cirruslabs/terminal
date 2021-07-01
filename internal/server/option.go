package server

import (
	"github.com/sirupsen/logrus"
	"net/http"
)

type Option func(*TerminalServer)

type WebsocketOriginFunc func(*http.Request) bool
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

func WithWebsocketOriginFunc(websocketOriginFunc WebsocketOriginFunc) Option {
	return func(ts *TerminalServer) {
		ts.websocketOriginFunc = websocketOriginFunc
	}
}

func WithLocatorGenerator(locatorGenerator LocatorGenerator) Option {
	return func(ts *TerminalServer) {
		ts.generateLocator = locatorGenerator
	}
}

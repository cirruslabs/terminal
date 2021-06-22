package host

import "github.com/sirupsen/logrus"

type Option func(*TerminalHost)

type LocatorCallback func(string) error

func WithLogger(logger *logrus.Logger) Option {
	return func(th *TerminalHost) {
		th.logger = logger
	}
}

func WithServerAddress(address string) Option {
	return func(th *TerminalHost) {
		th.serverAddress = address
	}
}

func WithServerInsecure() Option {
	return func(th *TerminalHost) {
		th.serverInsecure = true
	}
}

func WithTrustedSecret(trustedSecret string) Option {
	return func(th *TerminalHost) {
		th.trustedSecret = trustedSecret
	}
}

func WithLocatorCallback(locatorCallback LocatorCallback) Option {
	return func(th *TerminalHost) {
		th.locatorCallback = locatorCallback
	}
}

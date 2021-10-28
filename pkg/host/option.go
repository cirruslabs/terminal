package host

import (
	"go.uber.org/zap"
)

type Option func(*TerminalHost)

type LocatorCallback func(string) error

func WithLogger(logger *zap.Logger) Option {
	return func(th *TerminalHost) {
		th.logger = logger
	}
}

func WithServerAddress(address string) Option {
	return func(th *TerminalHost) {
		th.serverAddress = address
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

func WithShellEnv(shellEnv []string) Option {
	return func(th *TerminalHost) {
		th.shellEnv = shellEnv
	}
}

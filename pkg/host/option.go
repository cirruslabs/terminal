package host

type Option func(*TerminalHost)

type LocatorCallback func(string)

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

func WithTrustedSecret(trustedPassword string) Option {
	return func(th *TerminalHost) {
		th.trustedSecret = trustedPassword
	}
}

func WithLocatorCallback(locatorCallback LocatorCallback) Option {
	return func(th *TerminalHost) {
		th.locatorCallback = locatorCallback
	}
}

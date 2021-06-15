package terminal

type Option func(*Terminal)

func WithTrustedSecret(trustedSecret string) Option {
	return func(terminal *Terminal) {
		terminal.trustedSecret = trustedSecret
	}
}

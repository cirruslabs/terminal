package host

import "fmt"

func New(opts ...Option) (*TerminalHost, error) {
	return nil, fmt.Errorf("Cirrus Terminal doesn't support Windows yet, see https://github.com/creack/pty/pull/109")
}

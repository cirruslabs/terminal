package host

import (
	"context"
	"errors"
	"time"
)

var ErrUnsupported = errors.New("Cirrus Terminal doesn't support Windows yet, see https://github.com/creack/pty/pull/109")

func New(opts ...Option) (*TerminalHost, error) {
	return nil, ErrUnsupported
}

func (th *TerminalHost) Run(ctx context.Context) error {
	return ErrUnsupported
}

func (th *TerminalHost) LastActivity() time.Time {
	return time.Time{}
}

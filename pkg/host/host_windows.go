package host

import (
	"context"
	"errors"
	"github.com/cirruslabs/terminal/pkg/host/session"
	"time"
)

var ErrUnsupported = errors.New("Cirrus Terminal doesn't support Windows yet, see https://github.com/creack/pty/pull/109")

func New(opts ...Option) (*TerminalHost, error) {
	return nil, ErrUnsupported
}

func (th *TerminalHost) Run(ctx context.Context) error {
	return ErrUnsupported
}

func (th *TerminalHost) LastRegistration() time.Time {
	return time.Time{}
}

func (th *TerminalHost) LastActivity() time.Time {
	return time.Time{}
}

func (th *TerminalHost) NumSessions() int {
	return 0
}

func (th *TerminalHost) NumSessionsFunc(f func(session *session.Session) bool) int {
	return 0
}

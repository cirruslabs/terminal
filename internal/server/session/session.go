package session

import (
	"context"
	"github.com/cirruslabs/terminal/internal/api"
	"github.com/google/uuid"
)

type Session struct {
	//nolint:containedctx // seems perfectly valid for our use-case
	subCtx context.Context
	cancel context.CancelFunc

	token string

	requestedDimensions *api.TerminalDimensions

	TerminalInputChan    chan []byte
	TerminalOutputChan   chan []byte
	ChangeDimensionsChan chan *api.TerminalDimensions
}

func New(ctx context.Context, requestedDimensions *api.TerminalDimensions) *Session {
	subCtx, cancel := context.WithCancel(ctx)

	return &Session{
		subCtx:               subCtx,
		cancel:               cancel,
		token:                uuid.New().String(),
		requestedDimensions:  requestedDimensions,
		TerminalInputChan:    make(chan []byte),
		TerminalOutputChan:   make(chan []byte),
		ChangeDimensionsChan: make(chan *api.TerminalDimensions),
	}
}

func (session *Session) Token() string {
	return session.token
}

func (session *Session) RequestedDimensions() *api.TerminalDimensions {
	return session.requestedDimensions
}

func (session *Session) Context() context.Context {
	return session.subCtx
}

func (session *Session) Close() error {
	session.cancel()

	return nil
}

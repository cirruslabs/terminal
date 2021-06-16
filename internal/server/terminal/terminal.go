package terminal

import (
	"crypto/subtle"
	"fmt"
	"github.com/cirruslabs/terminal/internal/server/session"
	"sync"
)

type Terminal struct {
	locator string

	trustedSecret string

	sessionsLock   sync.RWMutex
	sessions       map[string]*session.Session
	noMoreSessions bool

	NewSessionChan chan *session.Session
}

func New(locator string, opts ...Option) *Terminal {
	terminal := &Terminal{
		locator:        locator,
		sessions:       make(map[string]*session.Session),
		NewSessionChan: make(chan *session.Session),
	}

	// Apply options
	for _, opt := range opts {
		opt(terminal)
	}

	return terminal
}

func (terminal *Terminal) RegisterSession(session *session.Session) error {
	terminal.sessionsLock.Lock()
	defer terminal.sessionsLock.Unlock()

	if terminal.noMoreSessions {
		return fmt.Errorf("refusing to register new session because terminal is shutting down")
	}

	if _, ok := terminal.sessions[session.Token()]; ok {
		return fmt.Errorf("attempted to register multiple sessions with the same token")
	}

	terminal.sessions[session.Token()] = session

	return nil
}

func (terminal *Terminal) UnregisterSession(session *session.Session) {
	terminal.sessionsLock.Lock()
	defer terminal.sessionsLock.Unlock()

	delete(terminal.sessions, session.Token())
}

func (terminal *Terminal) IsSecretValid(secret string) bool {
	if terminal.trustedSecret == "" {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(terminal.trustedSecret), []byte(secret)) == 1
}

func (terminal *Terminal) FindSession(token string) *session.Session {
	terminal.sessionsLock.RLock()
	defer terminal.sessionsLock.RUnlock()

	session, ok := terminal.sessions[token]
	if !ok {
		return nil
	}

	return session
}

func (terminal *Terminal) Locator() string {
	return terminal.locator
}

func (terminal *Terminal) Close() error {
	terminal.sessionsLock.Lock()
	defer terminal.sessionsLock.Unlock()

	terminal.noMoreSessions = true

	for token, session := range terminal.sessions {
		if err := session.Close(); err != nil {
			return err
		}

		delete(terminal.sessions, token)
	}

	return nil
}

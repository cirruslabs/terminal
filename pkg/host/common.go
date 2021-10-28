package host

import (
	"github.com/cirruslabs/terminal/pkg/host/session"
	"go.uber.org/zap"
	"sync"
	"time"
)

type TerminalHost struct {
	logger *zap.Logger

	shellEnv []string

	serverAddress string

	trustedSecret string

	locatorCallback LocatorCallback

	sessionsLock     sync.Mutex
	sessions         map[string]*session.Session
	lastRegistration time.Time
	lastActivity     time.Time
}

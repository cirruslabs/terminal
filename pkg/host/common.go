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

	lastConnectionMtx sync.Mutex
	lastConnection    time.Time

	sessionsLock     sync.Mutex
	sessions         map[string]*session.Session
	lastRegistration time.Time
	lastActivity     time.Time
}

package host

import (
	"github.com/cirruslabs/terminal/pkg/host/session"
	"github.com/sirupsen/logrus"
	"sync"
	"time"
)

type TerminalHost struct {
	logger *logrus.Logger

	serverAddress string

	trustedSecret string

	locatorCallback LocatorCallback

	sessionsLock sync.Mutex
	sessions     map[string]*session.Session
	lastActivity time.Time
}

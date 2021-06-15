package host

import (
	"github.com/sirupsen/logrus"
	"sync"
	"time"
)

type TerminalHost struct {
	logger *logrus.Logger

	serverAddress  string
	serverInsecure bool

	trustedSecret string

	locatorCallback LocatorCallback

	lastActivityLock sync.Mutex
	lastActivity time.Time
}

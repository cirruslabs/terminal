//go:build !windows
// +build !windows

//nolint:testpackage // we intentionally don't use a separate test package to call the registerSession() method
package host

import (
	"github.com/cirruslabs/terminal/pkg/host/session"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"testing"
	"time"
)

func TestNumSessionsNormalAndFunc(t *testing.T) {
	terminalHost, err := New(WithTrustedSecret("doesn't matter"))
	if err != nil {
		t.Fatal(err)
	}

	session1 := session.New(zap.NewNop(), "first one", nil)
	terminalHost.registerSession(session1)

	session2 := session.New(zap.NewNop(), "second one", nil)
	terminalHost.registerSession(session2)

	assert.Equal(t, 2, terminalHost.NumSessions())
	assert.Equal(t, 2, terminalHost.NumSessionsFunc(func(session *session.Session) bool {
		uninitializedTime := time.Time{}

		return session.LastActivity() == uninitializedTime
	}))
}

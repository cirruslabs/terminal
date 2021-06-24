// nolint:testpackage // we intentionally don't use a separate test package to call the updateLastActivity() method
package session

import (
	"github.com/cirruslabs/terminal/internal/api"
	"github.com/creack/pty"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestLastActivitySimple(t *testing.T) {
	session := New(nil, "")

	require.Equal(t, time.Time{}, session.LastActivity())

	session.updateLastActivity()
	require.WithinDuration(t, session.LastActivity(), time.Now(), time.Second)
}

func TestTerminalDimensionsToPtyWinsize(t *testing.T) {
	assert.Equal(t, &pty.Winsize{Rows: 24, Cols: 80},
		terminalDimensionsToPtyWinsize(nil))
	assert.Equal(t, &pty.Winsize{},
		terminalDimensionsToPtyWinsize(&api.TerminalDimensions{}))
	assert.Equal(t, &pty.Winsize{Rows: 48, Cols: 160},
		terminalDimensionsToPtyWinsize(&api.TerminalDimensions{WidthColumns: 160, HeightRows: 48}))
}

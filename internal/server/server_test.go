// nolint:testpackage // we intentionally don't use a separate test package to call internal TerminalServer methods
package server

import (
	"github.com/cirruslabs/terminal/internal/server/terminal"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestTerminalRegistrationUnregistration(t *testing.T) {
	terminalServer, err := New()
	if err != nil {
		t.Fatal(err)
	}

	terminal := terminal.New("doesn't matter")

	require.NoError(t, terminalServer.registerTerminal(terminal))
	require.NotNil(t, terminalServer.findTerminal(terminal.Locator()))

	terminalServer.unregisterTerminal(terminal)
	require.Nil(t, terminalServer.findTerminal(terminal.Locator()))
}

func TestNoDuplicateRegistrations(t *testing.T) {
	terminalServer, err := New()
	if err != nil {
		t.Fatal(err)
	}

	terminal := terminal.New("doesn't matter")

	require.NoError(t, terminalServer.registerTerminal(terminal))
	require.Error(t, terminalServer.registerTerminal(terminal))
}

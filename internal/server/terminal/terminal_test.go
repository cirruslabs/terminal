package terminal_test

import (
	"context"
	"github.com/cirruslabs/terminal/internal/server/session"
	"github.com/cirruslabs/terminal/internal/server/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestSecretValidation(t *testing.T) {
	const locator = "doesn't matter"
	const secret = "this is really a secret"

	var testCases = []struct {
		Name             string
		Terminal         *terminal.Terminal
		SecretToValidate string
		ShouldBeValid    bool
	}{
		{
			Name:             "valid secret",
			Terminal:         terminal.New(locator, terminal.WithTrustedSecret(secret)),
			SecretToValidate: secret,
			ShouldBeValid:    true,
		},
		{
			Name:             "empty trusted secret is never valid (empty)",
			Terminal:         terminal.New(locator, terminal.WithTrustedSecret("")),
			SecretToValidate: "",
			ShouldBeValid:    false,
		},
		{
			Name:             "empty trusted secret is never valid (non-empty)",
			Terminal:         terminal.New(locator, terminal.WithTrustedSecret("")),
			SecretToValidate: "123",
			ShouldBeValid:    false,
		},
		{
			Name:             "invalid secret (slightly longer)",
			Terminal:         terminal.New(locator, terminal.WithTrustedSecret(secret)),
			SecretToValidate: secret + "1",
			ShouldBeValid:    false,
		},
		{
			Name:             "invalid secret (different capitalization)",
			Terminal:         terminal.New(locator, terminal.WithTrustedSecret(secret)),
			SecretToValidate: strings.ToUpper(secret),
			ShouldBeValid:    false,
		},
		{
			Name:             "invalid secret (empty)",
			Terminal:         terminal.New(locator, terminal.WithTrustedSecret(secret)),
			SecretToValidate: "",
			ShouldBeValid:    false,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase

		t.Run(testCase.Name, func(t *testing.T) {
			actuallyValid := testCase.Terminal.IsSecretValid(testCase.SecretToValidate)

			assert.Equal(t, testCase.ShouldBeValid, actuallyValid)
		})
	}
}

func TestSessionsAreCleanedUpAfterTerminalClosure(t *testing.T) {
	terminal := terminal.New("doesn't matter")

	// Register a couple of sessions
	var registeredSessions []*session.Session
	const sessionsToRegister = 10

	for i := 0; i < sessionsToRegister; i++ {
		newSession := session.New(context.Background(), nil)

		require.NoError(t, terminal.RegisterSession(newSession))
		require.Equal(t, newSession, terminal.FindSession(newSession.Token()))

		registeredSessions = append(registeredSessions, newSession)
	}

	// Close the terminal
	err := terminal.Close()
	require.NoError(t, err)

	// Ensure the sessions were cleaned up
	for i, registeredSession := range registeredSessions {
		select {
		case <-registeredSession.Context().Done():
			// The session was closed, good!
		default:
			t.Fatalf("session %d wasn't closed", i)
		}

		require.Nil(t, terminal.FindSession(registeredSession.Token()))
	}
}

func TestNoSessionRegistrationAfterTerminalClosure(t *testing.T) {
	terminal := terminal.New("doesn't matter")

	// Close the terminal
	require.NoError(t, terminal.Close())

	// Try to register a new session
	session := session.New(context.Background(), nil)
	require.Error(t, terminal.RegisterSession(session))
	require.Nil(t, terminal.FindSession(session.Token()))
}

func TestSessionRegistrationUnregistration(t *testing.T) {
	terminal := terminal.New("doesn't matter")
	session := session.New(context.Background(), nil)

	// Register session
	require.NoError(t, terminal.RegisterSession(session))
	require.Equal(t, session, terminal.FindSession(session.Token()))

	// Unregister session
	terminal.UnregisterSession(session)
	require.Nil(t, terminal.FindSession(session.Token()))
}

func TestNoDuplicateRegistrations(t *testing.T) {
	terminal := terminal.New("doesn't matter")

	session := session.New(context.Background(), nil)
	require.NoError(t, terminal.RegisterSession(session))
	require.Error(t, terminal.RegisterSession(session))
}

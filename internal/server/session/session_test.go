package session_test

import (
	"context"
	"github.com/cirruslabs/terminal/internal/server/session"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSessionCloseResultsInContextCancellation(t *testing.T) {
	session := session.New(context.Background(), nil)
	require.NoError(t, session.Close())

	select {
	case <-session.Context().Done():
		return
	default:
		t.Fatal("session's context wasn't cancelled after session.Close()")
	}
}

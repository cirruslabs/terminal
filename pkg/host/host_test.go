// +build !windows

package host

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestLastActivitySimple(t *testing.T) {
	terminalHost, err := New(WithTrustedSecret("doesn't matter for this test"))
	if err != nil {
		t.Fatal(err)
	}

	require.Equal(t, time.Time{}, terminalHost.LastActivity())

	terminalHost.updateLastActivity()
	require.WithinDuration(t, terminalHost.LastActivity(), time.Now(), time.Second)
}

package session_test

import (
	"bytes"
	"fmt"
	"github.com/cirruslabs/terminal/pkg/host/session"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"io"
	"os"
	"testing"
)

func TestEnvPassthrough(t *testing.T) {
	if err := os.Setenv("TEST_ENV_PASSTHROUGH_CANARY", "some value"); err != nil {
		t.Fatal(err)
	}

	shellPty, err := session.NewShellPTY(zap.NewNop().Sugar(), nil)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := fmt.Fprintln(shellPty, "env ; exit"); err != nil {
		t.Fatal(err)
	}

	buf := bytes.NewBuffer([]byte{})

	if _, err := io.Copy(buf, shellPty); err != nil {
		t.Fatal(err)
	}

	if err := shellPty.Close(); err != nil {
		t.Fatal(err)
	}

	assert.Contains(t, buf.String(), "TEST_ENV_PASSTHROUGH_CANARY=some value")
}

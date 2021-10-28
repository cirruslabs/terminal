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

	shellPty, err := session.NewShellPTY(zap.NewNop().Sugar(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := fmt.Fprintln(shellPty, "env ; exit"); err != nil {
		t.Fatal(err)
	}

	buf := bytes.NewBuffer([]byte{})

	_, _ = io.Copy(buf, shellPty)

	if err := shellPty.Close(); err != nil {
		t.Fatal(err)
	}

	assert.Contains(t, buf.String(), "TEST_ENV_PASSTHROUGH_CANARY=some value")
}

func TestEnvCustom(t *testing.T) {
	shellPty, err := session.NewShellPTY(zap.NewNop().Sugar(), nil, []string{"TEST_ENV_PASSTHROUGH_CANARY=some value"})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := fmt.Fprintln(shellPty, "env ; exit"); err != nil {
		t.Fatal(err)
	}

	buf := bytes.NewBuffer([]byte{})

	_, _ = io.Copy(buf, shellPty)

	if err := shellPty.Close(); err != nil {
		t.Fatal(err)
	}

	assert.Contains(t, buf.String(), "TEST_ENV_PASSTHROUGH_CANARY=some value")
}

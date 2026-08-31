//go:build darwin || linux || freebsd || openbsd || netbsd

package app

import (
	"bytes"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestRunRejectsFIFOWithoutBlocking(t *testing.T) {
	path := t.TempDir() + "/diagram.fifo"
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan invocation, 1)
	go func() {
		var stdout, stderr bytes.Buffer
		code := Run([]string{"-f", path}, strings.NewReader(""), &stdout, &stderr)
		done <- invocation{code: code, stdout: stdout.String(), stderr: stderr.String()}
	}()

	select {
	case got := <-done:
		if got.code != 1 || got.stdout != "" || !strings.Contains(got.stderr, "regular file") {
			t.Fatalf("result=%+v", got)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("FIFO input blocked before regular-file validation")
	}
}

package apply

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func openBody(t *testing.T, body string) (*os.File, string) {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	if err := os.WriteFile(src, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	in, err := os.Open(src)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { in.Close() })
	return in, filepath.Join(dir, "dst")
}

func TestCloneStaysIndependentOfSource(t *testing.T) {
	t.Parallel()
	in, dst := openBody(t, "hello")
	if err := clone(in, dst); err != nil {
		if errors.Is(err, unix.ENOTSUP) || errors.Is(err, errors.ErrUnsupported) {
			t.Skipf("volume can't clone: %v", err)
		}
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(in.Name())
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "hello" {
		t.Errorf("source body = %q, want it untouched", body)
	}
}

func TestCopyToIsTheFallbackForClone(t *testing.T) {
	t.Parallel()
	in, dst := openBody(t, "hello")
	if err := copyTo(in, dst); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "hello" {
		t.Errorf("body = %q", body)
	}
}

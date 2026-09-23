package launchd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wjames111/ccdejavu/internal/paths"
)

func TestPlistRoundTrip(t *testing.T) {
	t.Parallel()
	body, err := renderPlist("/opt/bin/ccdejavu", "/h/Library/Logs/ccdejavu")
	if err != nil {
		t.Fatal(err)
	}
	got, err := parseProgram(body)
	if err != nil || got != "/opt/bin/ccdejavu" {
		t.Fatalf("parseProgram = %q, %v", got, err)
	}
	for _, want := range []string{Label, "<string>watch</string>", "/h/Library/Logs/ccdejavu/ccdejavu.err.log", "<key>KeepAlive</key>"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("plist is missing %q", want)
		}
	}
}

func TestInstalledProgramReadsThePlist(t *testing.T) {
	t.Parallel()
	p := paths.Paths{Home: t.TempDir()}
	body, err := renderPlist("/opt/homebrew/Cellar/ccdejavu/0.1.0/bin/ccdejavu", p.Logs())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(p.LaunchAgents(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(DefinitionPath(p), body, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := InstalledProgram(p)
	if err != nil || got != "/opt/homebrew/Cellar/ccdejavu/0.1.0/bin/ccdejavu" {
		t.Fatalf("InstalledProgram = %q, %v", got, err)
	}
}

func TestIsTransientPath(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		filepath.Join(t.TempDir(), "ccdejavu"):                 true,
		"/private/var/folders/x/go-build123/b001/exe/ccdejavu": true,
		"/opt/homebrew/Cellar/ccdejavu/0.1.0/bin/ccdejavu":     false,
		"/Users/me/Desktop/repos/ccdejavu/ccdejavu":            false,
		"/tmp/ccdejavu":                  true,
		"/private/tmp/ccdejavu":          true,
		"/Users/me/go-builders/ccdejavu": false,
	}
	if r, err := filepath.EvalSymlinks(t.TempDir()); err == nil {
		cases[filepath.Join(r, "ccdejavu")] = true
	}
	for path, want := range cases {
		if got := IsTransientPath(path); got != want {
			t.Errorf("IsTransientPath(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestIsRunning(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		"gui/501/io.github.wjames111.ccdejavu = {\n\tactive count = 1\n\tstate = running\n\n\tprogram = /x\n}\n":     true,
		"gui/501/io.github.wjames111.ccdejavu = {\n\tactive count = 1\n\tstate = not running\n\n\tprogram = /x\n}\n": false,
		"x = {\n\tendpoints = {\n\t\tstate = running\n\t}\n\tstate = spawn scheduled\n}\n":                           false,
		"": false,
	}
	for out, want := range cases {
		if got := isRunning([]byte(out)); got != want {
			t.Errorf("isRunning(%q) = %v, want %v", out, got, want)
		}
	}
}

func lintPlist(t *testing.T, body []byte) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ccdejavu.plist")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.CommandContext(t.Context(), "plutil", "-lint", path).CombinedOutput()
	if err != nil {
		t.Fatalf("plutil -lint: %v: %s", err, out)
	}
}

func TestPlistEscapesXML(t *testing.T) {
	t.Parallel()
	body, err := renderPlist("/opt/bin/ccdejavu", "/h/Library/Logs/ccdejavu")
	if err != nil {
		t.Fatal(err)
	}
	lintPlist(t, body)

	binary := "/Users/a&b/bin/ccdejavu"
	logDir := "/Users/a&b/Library/Logs/ccdejavu"
	body, err = renderPlist(binary, logDir)
	if err != nil {
		t.Fatal(err)
	}
	lintPlist(t, body)

	got, err := parseProgram(body)
	if err != nil || got != binary {
		t.Fatalf("parseProgram = %q, %v, want %q", got, err, binary)
	}
}

func TestResolveBinaryRefusesATempBuild(t *testing.T) {
	t.Parallel()
	bin := filepath.Join(t.TempDir(), "ccdejavu")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := resolveBinary(bin)
	if err == nil || !strings.Contains(err.Error(), "temporary build") {
		t.Fatalf("err = %v, want a temporary build refusal", err)
	}
}

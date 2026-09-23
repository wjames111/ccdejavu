// Package launchd installs ccdejavu's watcher as a per-user LaunchAgent.
package launchd

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/wjames111/ccdejavu/internal/paths"
)

// Label is the LaunchAgent identifier.
const Label = "io.github.wjames111.ccdejavu"

// xmlEscape escapes text for safe placement inside plist XML text nodes.
func xmlEscape(s string) (string, error) {
	var buf bytes.Buffer
	if err := xml.EscapeText(&buf, []byte(s)); err != nil {
		return "", err
	}
	return buf.String(), nil
}

var plistTemplate = template.Must(template.New("plist").Funcs(template.FuncMap{"xml": xmlEscape}).Parse(
	`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>{{.Label}}</string>
	<key>ProgramArguments</key>
	<array>
		<string>{{xml .Binary}}</string>
		<string>watch</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
	<key>ProcessType</key>
	<string>Background</string>
	<key>StandardOutPath</key>
	<string>{{xml .LogDir}}/ccdejavu.log</string>
	<key>StandardErrorPath</key>
	<string>{{xml .LogDir}}/ccdejavu.err.log</string>
</dict>
</plist>
`))

// DefinitionPath is where the LaunchAgent plist lives.
func DefinitionPath(p paths.Paths) string {
	return filepath.Join(p.LaunchAgents(), Label+".plist")
}

func renderPlist(binary, logDir string) ([]byte, error) {
	var buf bytes.Buffer
	err := plistTemplate.Execute(&buf, map[string]string{
		"Label":  Label,
		"Binary": binary,
		"LogDir": logDir,
	})
	return buf.Bytes(), err
}

// Install writes the plist and loads the agent, replacing any earlier one.
func Install(ctx context.Context, p paths.Paths, binary string) error {
	abs, err := resolveBinary(binary)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(p.Logs(), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(p.LaunchAgents(), 0o755); err != nil {
		return err
	}
	body, err := renderPlist(abs, p.Logs())
	if err != nil {
		return err
	}
	if err := os.WriteFile(DefinitionPath(p), body, 0o644); err != nil {
		return err
	}
	_ = bootout(ctx)
	waitUnloaded(ctx)
	return run(ctx, "launchctl", "bootstrap", domain(), DefinitionPath(p))
}

// waitUnloaded polls until the agent is gone or 10s pass: bootout returns
// before the old process exits, and bootstrapping too soon fails with an I/O error.
func waitUnloaded(ctx context.Context) {
	deadline := time.After(10 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if exec.CommandContext(ctx, "launchctl", "print", domainTarget()).Run() != nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-deadline:
			return
		case <-ticker.C:
		}
	}
}

// Uninstall stops the agent and removes its plist.
func Uninstall(ctx context.Context, p paths.Paths) error {
	_ = bootout(ctx)
	if err := os.Remove(DefinitionPath(p)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// isRunning reads the top-level state line from `launchctl print` output.
func isRunning(out []byte) bool {
	return bytes.Contains(out, []byte("\n\tstate = running\n"))
}

// Running reports whether the watcher process is up, not just loaded.
func Running(ctx context.Context) bool {
	out, err := exec.CommandContext(ctx, "launchctl", "print", domainTarget()).CombinedOutput()
	return err == nil && isRunning(out)
}

var programArgsRe = regexp.MustCompile(`(?s)<key>ProgramArguments</key>\s*<array>\s*<string>([^<]*)</string>`)

func parseProgram(raw []byte) (string, error) {
	m := programArgsRe.FindSubmatch(raw)
	if m == nil {
		return "", fmt.Errorf("launchd: no ProgramArguments in plist")
	}
	return html.UnescapeString(string(m[1])), nil
}

// InstalledProgram reports the binary the installed agent runs. Homebrew puts
// each version in its own Cellar folder, so after an upgrade the agent keeps running
// the old binary until `ccdejavu install` is run again.
func InstalledProgram(p paths.Paths) (string, error) {
	raw, err := os.ReadFile(DefinitionPath(p))
	if err != nil {
		return "", err
	}
	return parseProgram(raw)
}

// goBuildRe matches go's temporary build output folders, e.g. /go-build123456.
var goBuildRe = regexp.MustCompile(`/go-build[0-9]`)

// IsTransientPath reports whether a binary lives in a temp or go-build folder
// that won't survive the process exiting, like a `go run` build.
func IsTransientPath(path string) bool {
	sep := string(os.PathSeparator)
	candidates := []string{path}
	if r, err := filepath.EvalSymlinks(path); err == nil && r != path {
		candidates = append(candidates, r)
	}
	for _, c := range candidates {
		if goBuildRe.MatchString(c) {
			return true
		}
	}
	tmps := []string{filepath.Clean(os.TempDir()), "/tmp", "/private/tmp"}
	if r, err := filepath.EvalSymlinks(tmps[0]); err == nil {
		if r = filepath.Clean(r); r != tmps[0] {
			tmps = append(tmps, r)
		}
	}
	for _, tmp := range tmps {
		if tmp == sep {
			continue
		}
		for _, c := range candidates {
			if strings.HasPrefix(c, tmp+sep) {
				return true
			}
		}
	}
	return false
}

func resolveBinary(binary string) (string, error) {
	abs, err := filepath.Abs(binary)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(abs); err != nil {
		return "", fmt.Errorf("launchd: binary %s: %w", abs, err)
	}
	if IsTransientPath(abs) {
		return "", fmt.Errorf("launchd: refusing to install %s: it is a temporary build that "+
			"will be deleted, leaving the LaunchAgent pointing at nothing. "+
			"Build a real binary instead: `task build && ./ccdejavu install`", abs)
	}
	return abs, nil
}

func bootout(ctx context.Context) error { return run(ctx, "launchctl", "bootout", domainTarget()) }

func domain() string { return "gui/" + strconv.Itoa(os.Getuid()) }

func domainTarget() string { return domain() + "/" + Label }

// run folds a command's output into its error so launchctl explains itself.
func run(ctx context.Context, name string, args ...string) error {
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

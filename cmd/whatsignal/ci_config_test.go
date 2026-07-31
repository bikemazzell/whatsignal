package main

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestScannerConfigsExcludeWorktrees(t *testing.T) {
	files := map[string]string{
		"Makefile":          "../../Makefile",
		"security workflow": "../../.github/workflows/security.yml",
	}

	for name, path := range files {
		t.Run(name, func(t *testing.T) {
			content, err := os.ReadFile(path)
			require.NoError(t, err)

			require.Contains(t, string(content), ".worktrees")
		})
	}

	makefile, err := os.ReadFile("../../Makefile")
	require.NoError(t, err)
	require.True(t, strings.Contains(string(makefile), "-exclude-dir=.worktrees"))

	workflow, err := os.ReadFile("../../.github/workflows/security.yml")
	require.NoError(t, err)
	require.True(t, strings.Contains(string(workflow), "-exclude-dir=.worktrees"))
}

// goVersionIn extracts the single Go patch version a file declares. An absent pattern
// fails the test rather than yielding an empty string, so a renamed field or reformatted
// file surfaces as a failure instead of silently passing with nothing to compare.
func goVersionIn(t *testing.T, path string, pattern *regexp.Regexp) string {
	t.Helper()

	content, err := os.ReadFile(path)
	require.NoError(t, err)

	matches := pattern.FindAllStringSubmatch(string(content), -1)
	require.NotEmpty(t, matches, "no Go version found in %s using %s", path, pattern)

	version := matches[0][1]
	for _, m := range matches[1:] {
		require.Equal(t, version, m[1], "%s declares conflicting Go versions", path)
	}
	return version
}

// go.mod is the single source of truth. Every other pin must agree with it, so a
// toolchain bump that misses a file fails here rather than in CI on a different runner.
func TestToolchainPatchVersionIsConsistent(t *testing.T) {
	want := goVersionIn(t, "../../go.mod", regexp.MustCompile(`(?m)^go (\d+\.\d+\.\d+)$`))

	files := map[string]struct {
		path    string
		pattern *regexp.Regexp
	}{
		"Dockerfile": {
			"../../Dockerfile",
			regexp.MustCompile(`golang:(\d+\.\d+\.\d+)-alpine`),
		},
		"security workflow": {
			"../../.github/workflows/security.yml",
			regexp.MustCompile(`GO_VERSION: '(\d+\.\d+\.\d+)'`),
		},
		"integration workflow": {
			"../../.github/workflows/integration-tests.yml",
			regexp.MustCompile(`GO_VERSION: '(\d+\.\d+\.\d+)'`),
		},
	}

	for name, f := range files {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, want, goVersionIn(t, f.path, f.pattern),
				"%s does not match the go directive in go.mod", name)
		})
	}
}

func TestBinaryHealthcheckUsesReadinessEndpoint(t *testing.T) {
	content, err := os.ReadFile("../../cmd/whatsignal/main.go")
	require.NoError(t, err)

	text := string(content)
	require.Contains(t, text, "localhost:8082/readyz")
	require.NotContains(t, text, "localhost:8082/health")
}

package main

import (
	"os"
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

func TestToolchainPatchVersionIsConsistent(t *testing.T) {
	const fixedGoVersion = "1.26.4"

	files := map[string]string{
		"go.mod":               "../../go.mod",
		"Dockerfile":           "../../Dockerfile",
		"security workflow":    "../../.github/workflows/security.yml",
		"integration workflow": "../../.github/workflows/integration-tests.yml",
	}

	for name, path := range files {
		t.Run(name, func(t *testing.T) {
			content, err := os.ReadFile(path)
			require.NoError(t, err)

			text := string(content)
			require.Contains(t, text, fixedGoVersion)
			require.NotContains(t, text, "1.26.3")
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

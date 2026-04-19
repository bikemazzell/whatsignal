package config

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestDeploymentExamplesDeclareEncryptionSalts(t *testing.T) {
	repoRoot := filepath.Join("..", "..")
	files := []string{
		".env.example",
		"docker-compose.yml",
	}

	for _, name := range files {
		t.Run(name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join(repoRoot, name))
			require.NoError(t, err)

			text := string(content)
			assert.Contains(t, text, "WHATSIGNAL_ENCRYPTION_SALT")
			assert.Contains(t, text, "WHATSIGNAL_ENCRYPTION_LOOKUP_SALT")
		})
	}
}

func TestDeploymentExamplesDeclareAdminToken(t *testing.T) {
	repoRoot := filepath.Join("..", "..")
	files := []string{
		".env.example",
		"docker-compose.yml",
	}

	for _, name := range files {
		t.Run(name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join(repoRoot, name))
			require.NoError(t, err)

			assert.Contains(t, string(content), "WHATSIGNAL_ADMIN_TOKEN")
		})
	}
}

func TestSetupScriptsGenerateEncryptionSalts(t *testing.T) {
	repoRoot := filepath.Join("..", "..")
	files := []string{
		"scripts/setup.sh",
		"scripts/deploy.sh",
	}

	for _, name := range files {
		t.Run(name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join(repoRoot, name))
			require.NoError(t, err)

			text := string(content)
			assert.True(t, strings.Count(text, "WHATSIGNAL_ENCRYPTION_SALT") >= 2, "script should generate and write encryption salt")
			assert.True(t, strings.Count(text, "WHATSIGNAL_ENCRYPTION_LOOKUP_SALT") >= 2, "script should generate and write lookup salt")
			assert.True(t, strings.Count(text, "WHATSIGNAL_ADMIN_TOKEN") >= 2, "script should generate and write admin token")
		})
	}
}

func TestDockerfileSecurityStructure(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "Dockerfile"))
	require.NoError(t, err)

	dockerfile := string(content)
	fromPattern := regexp.MustCompile(`(?m)^FROM\s+([^\s]+)(?:\s+AS\s+(\S+))?`)
	froms := fromPattern.FindAllStringSubmatch(dockerfile, -1)
	require.Len(t, froms, 2, "Dockerfile should have builder and runtime stages")

	for _, from := range froms {
		assert.Contains(t, from[1], "@sha256:", "base image %q must be digest-pinned", from[1])
	}

	finalStage := dockerfile[strings.LastIndex(dockerfile, "\nFROM ")+1:]
	assert.Contains(t, finalStage, "FROM gcr.io/distroless/static-debian12:nonroot@sha256:")
	assert.Contains(t, finalStage, "\nUSER nonroot:nonroot\n")
	assert.NotContains(t, finalStage, "\nRUN ", "final stage should not install packages or use a shell")
	assert.NotContains(t, finalStage, " apk ")
	assert.NotContains(t, finalStage, " apt-get ")
}

func TestDockerComposeSecurityStructure(t *testing.T) {
	compose := loadComposeFile(t)
	services := compose["services"].(map[string]interface{})

	whatsignal := services["whatsignal"].(map[string]interface{})
	assert.Equal(t, true, whatsignal["read_only"])
	assert.Contains(t, asStringSlice(whatsignal["cap_drop"]), "ALL")
	assert.Contains(t, asStringSlice(whatsignal["security_opt"]), "no-new-privileges:true")
	if user, ok := whatsignal["user"]; ok {
		assert.Equal(t, "65532:65532", user)
	}
	env := asStringSlice(whatsignal["environment"])
	for _, required := range []string{
		"WHATSAPP_API_KEY=${WHATSAPP_API_KEY}",
		"WHATSIGNAL_WHATSAPP_WEBHOOK_SECRET=${WHATSIGNAL_WHATSAPP_WEBHOOK_SECRET}",
		"WHATSIGNAL_ADMIN_TOKEN=${WHATSIGNAL_ADMIN_TOKEN}",
		"WHATSIGNAL_ENCRYPTION_SECRET=${WHATSIGNAL_ENCRYPTION_SECRET}",
		"WHATSIGNAL_ENCRYPTION_SALT=${WHATSIGNAL_ENCRYPTION_SALT}",
		"WHATSIGNAL_ENCRYPTION_LOOKUP_SALT=${WHATSIGNAL_ENCRYPTION_LOOKUP_SALT}",
	} {
		assert.Contains(t, env, required)
	}

	for _, serviceName := range []string{"waha", "signal-cli-rest-api"} {
		service := services[serviceName].(map[string]interface{})
		assert.NotContains(t, service, "ports", "%s must not publish ports to the host by default", serviceName)
		assert.NotEmpty(t, service["expose"], "%s should expose its port only to the Compose network", serviceName)
	}
}

func TestDockerComposeDoesNotProvidePlaceholderSecrets(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "docker-compose.yml"))
	require.NoError(t, err)

	assert.NotContains(t, string(content), "your-api-key", "Compose must not provide placeholder credentials")
}

func loadComposeFile(t *testing.T) map[string]interface{} {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("..", "..", "docker-compose.yml"))
	require.NoError(t, err)

	var parsed map[string]interface{}
	require.NoError(t, yaml.Unmarshal(content, &parsed))
	return parsed
}

func asStringSlice(value interface{}) []string {
	items, ok := value.([]interface{})
	if !ok {
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		if text, ok := item.(string); ok {
			result = append(result, text)
		}
	}
	return result
}

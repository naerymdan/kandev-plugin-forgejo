package plugin

import (
	"os"
	"path/filepath"
	"testing"

	"kandev-plugin-forgejo/internal/sourcecontrol"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// manifest mirrors only the fields these tests assert on.
type manifest struct {
	ID                  string   `yaml:"id"`
	APIVersion          int      `yaml:"api_version"`
	Version             string   `yaml:"version"`
	MinKandevVersion    string   `yaml:"min_kandev_version"`
	RepositoryProviders []string `yaml:"repository_providers"`
	Capabilities        struct {
		APIRead  []string `yaml:"api_read"`
		APIWrite []string `yaml:"api_write"`
		State    bool     `yaml:"state"`
	} `yaml:"capabilities"`
	Actions []struct {
		Key          string `yaml:"key"`
		Scope        string `yaml:"scope"`
		MaxBodyBytes int    `yaml:"max_body_bytes"`
	} `yaml:"actions"`
	ReferenceSources []struct {
		Source   string `yaml:"source"`
		Provider string `yaml:"provider"`
		Kind     string `yaml:"kind"`
	} `yaml:"reference_sources"`
	Runtime struct {
		Type        string            `yaml:"type"`
		Executables map[string]string `yaml:"executables"`
	} `yaml:"runtime"`
	UI struct {
		Bundle string   `yaml:"bundle"`
		Styles []string `yaml:"styles"`
	} `yaml:"ui"`
	ConfigSchema struct {
		Required   []string `yaml:"required"`
		Properties map[string]struct {
			Type   string `yaml:"type"`
			Secret bool   `yaml:"secret"`
		} `yaml:"properties"`
	} `yaml:"config_schema"`
}

func loadManifest(t *testing.T) manifest {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "manifest.yaml"))
	require.NoError(t, err)
	var parsed manifest
	require.NoError(t, yaml.Unmarshal(raw, &parsed))
	return parsed
}

// The manifest declares the provider; the code implements it. A mismatch makes
// every native surface silently inert, so assert they agree.
func TestManifestProviderMatchesCode(t *testing.T) {
	t.Parallel()
	parsed := loadManifest(t)
	require.Equal(t, "kandev-plugin-forgejo", parsed.ID)
	require.Equal(t, []string{ProviderID}, parsed.RepositoryProviders)
	require.Len(t, parsed.ReferenceSources, 1)
	require.Equal(t, ReferenceSource, parsed.ReferenceSources[0].Source,
		"the manifest reference source and the extension's ReferenceSource must match exactly")
	require.Equal(t, ProviderID, parsed.ReferenceSources[0].Provider)
	require.Equal(t, "pull_request", parsed.ReferenceSources[0].Kind)
}

// Every action the recipe routes must be declared, or Kandev rejects the call
// before the handler runs.
func TestManifestDeclaresEveryRoutedAction(t *testing.T) {
	t.Parallel()
	parsed := loadManifest(t)
	declared := map[string]string{}
	for _, action := range parsed.Actions {
		require.NotEmpty(t, action.Scope, "action %q needs a scope", action.Key)
		require.Positive(t, action.MaxBodyBytes, "action %q needs a body limit", action.Key)
		declared[action.Key] = action.Scope
	}

	for key, wantScope := range map[string]string{
		sourcecontrol.ActionRepositoriesList:          "workspace",
		sourcecontrol.ActionRepositoriesInspect:       "workspace",
		sourcecontrol.ActionRepositoriesBranches:      "workspace",
		sourcecontrol.ActionChangeRequestsCreate:      "task",
		sourcecontrol.ActionChangeRequestsGet:         "task",
		sourcecontrol.ActionChangeRequestsLink:        "task",
		sourcecontrol.ActionChangeRequestsUnlink:      "task",
		sourcecontrol.ActionChangeRequestAssociations: "workspace",
		ActionConnectionGet:                           "workspace",
		ActionConnectionTest:                          "workspace",
		ActionConnectionSetEnabled:                    "workspace",
	} {
		scope, ok := declared[key]
		require.True(t, ok, "manifest does not declare routed action %q", key)
		require.Equal(t, wantScope, scope, "action %q has the wrong scope", key)
	}
	require.Len(t, parsed.Actions, 11, "an undeclared or stale action entry drifted from the routed set")
}

// The source-control contracts first shipped in v0.88.0. A lower floor would
// let the plugin install onto a host that cannot serve it.
func TestManifestPinsContractFloor(t *testing.T) {
	t.Parallel()
	parsed := loadManifest(t)
	require.Equal(t, "0.88.0", parsed.MinKandevVersion)
	require.Equal(t, 1, parsed.APIVersion)
	require.Equal(t, "binary", parsed.Runtime.Type)
}

// Least privilege: the plugin must not claim capabilities it never exercises.
func TestManifestClaimsOnlyUsedCapabilities(t *testing.T) {
	t.Parallel()
	parsed := loadManifest(t)
	require.ElementsMatch(t, []string{"tasks", "repositories"}, parsed.Capabilities.APIRead,
		"api_read backs only the attached-repository resolver and task->workspace lookup")
	require.Empty(t, parsed.Capabilities.APIWrite, "Kandev owns every entity mutation; this plugin writes none")
	require.True(t, parsed.Capabilities.State, "associations are stored in Host state")
}

// The packaged binaries must exist for every platform the manifest promises.
func TestManifestExecutablesCoverAllPlatforms(t *testing.T) {
	t.Parallel()
	parsed := loadManifest(t)
	for _, platform := range []string{
		"linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64", "windows-amd64",
	} {
		path, ok := parsed.Runtime.Executables[platform]
		require.True(t, ok, "manifest does not declare an executable for %s", platform)
		require.NotEmpty(t, path)
	}
	require.Equal(t, "server/plugin-windows-amd64.exe", parsed.Runtime.Executables["windows-amd64"],
		"the Windows executable needs its .exe suffix")
}

// The token is an operator credential and must be vaulted, not plain config.
func TestManifestMarksTokenSecret(t *testing.T) {
	t.Parallel()
	parsed := loadManifest(t)
	require.ElementsMatch(t, []string{"base_url", "api_token"}, parsed.ConfigSchema.Required)
	require.True(t, parsed.ConfigSchema.Properties["api_token"].Secret,
		"api_token must be a secret field so it is stored encrypted and masked")
	require.False(t, parsed.ConfigSchema.Properties["base_url"].Secret)
}

// The UI assets the manifest points at must actually exist in the repo, or the
// packaged plugin registers nothing in the browser.
func TestManifestUIAssetsExist(t *testing.T) {
	t.Parallel()
	parsed := loadManifest(t)
	require.Equal(t, "/ui/bundle.js", parsed.UI.Bundle)
	for _, asset := range append([]string{parsed.UI.Bundle}, parsed.UI.Styles...) {
		_, err := os.Stat(filepath.Join("..", "..", filepath.FromSlash(asset)))
		require.NoError(t, err, "manifest references missing UI asset %q", asset)
	}
}

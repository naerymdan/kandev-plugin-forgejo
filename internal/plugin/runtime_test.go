package plugin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"kandev-plugin-forgejo/internal/sourcecontrol"

	"github.com/kandev/kandev/pkg/pluginsdk"
	"github.com/stretchr/testify/require"
)

// configHost is a Host that only serves plugin config; every other Host method
// keeps its unimplemented default.
type configHost struct {
	pluginsdk.UnimplementedHostData
	config map[string]any
}

func (h *configHost) GetConfig(context.Context) (map[string]any, error) { return h.config, nil }
func (h *configHost) GetState(context.Context, string, string, string) (map[string]any, bool, error) {
	return nil, false, nil
}
func (h *configHost) SetState(context.Context, string, string, string, map[string]any) error {
	return nil
}
func (h *configHost) DeleteState(context.Context, string, string, string) error { return nil }
func (h *configHost) ListState(context.Context, string, string) ([]pluginsdk.StateEntry, error) {
	return nil, nil
}
func (h *configHost) RevealSecret(context.Context, string) (string, error) { return "", nil }
func (h *configHost) GetSecret(context.Context, string) (string, bool, error) {
	return "", false, nil
}
func (h *configHost) SetSecret(context.Context, string, string) error         { return nil }
func (h *configHost) DeleteSecret(context.Context, string) error              { return nil }
func (h *configHost) EmitEvent(context.Context, string, map[string]any) error { return nil }
func (h *configHost) InvokeUtilityAgent(context.Context, string, ...pluginsdk.UtilityAgentOptions) (string, error) {
	return "", nil
}

func decodeBody(t *testing.T, response *pluginsdk.PluginActionResponse) map[string]any {
	t.Helper()
	require.Equal(t, "application/json", response.Headers["Content-Type"])
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(response.Body, &decoded))
	return decoded
}

func TestConnectionGetReportsUnconfigured(t *testing.T) {
	t.Parallel()
	runtime := NewRuntime()
	runtime.SetHost(&configHost{config: map[string]any{}})

	response, err := runtime.HandleAction(context.Background(), &pluginsdk.PluginActionRequest{
		ActionKey: ActionConnectionGet,
		Context:   pluginsdk.VerifiedActionContext{WorkspaceID: "workspace-1"},
	})
	require.NoError(t, err)
	body := decodeBody(t, response)
	require.Equal(t, false, body["configured"])
	require.Equal(t, false, body["connected"])
	require.Contains(t, body["message"], "Settings > Plugins")
}

func TestConnectionTestReportsFlavorAndNeverLeaksToken(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		name       string
		version    string
		wantFlavor string
	}{
		{name: "forgejo", version: "13.0.5+gitea-1.22.0", wantFlavor: "forgejo"},
		{name: "gitea", version: "1.24.7", wantFlavor: "gitea"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/api/v1/user":
					_, _ = w.Write([]byte(`{"login":"kandev"}`))
				case "/api/v1/version":
					_ = json.NewEncoder(w).Encode(map[string]any{"version": testCase.version})
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()

			runtime := NewRuntime()
			runtime.SetHost(&configHost{config: map[string]any{
				"base_url": server.URL, "api_token": "super-secret-token",
			}})

			response, err := runtime.HandleAction(context.Background(), &pluginsdk.PluginActionRequest{
				ActionKey: ActionConnectionTest,
				Context:   pluginsdk.VerifiedActionContext{WorkspaceID: "workspace-1"},
			})
			require.NoError(t, err)
			body := decodeBody(t, response)
			require.Equal(t, true, body["configured"])
			require.Equal(t, true, body["connected"])
			require.Equal(t, "kandev", body["account"])
			require.Equal(t, testCase.wantFlavor, body["flavor"])
			require.NotContains(t, string(response.Body), "super-secret-token",
				"a status response must never echo the configured token")
		})
	}
}

func TestConnectionTestReportsRejectedToken(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"token super-secret-token is invalid"}`))
	}))
	defer server.Close()

	runtime := NewRuntime()
	runtime.SetHost(&configHost{config: map[string]any{"base_url": server.URL, "api_token": "super-secret-token"}})

	response, err := runtime.HandleAction(context.Background(), &pluginsdk.PluginActionRequest{
		ActionKey: ActionConnectionTest,
		Context:   pluginsdk.VerifiedActionContext{WorkspaceID: "workspace-1"},
	})
	require.NoError(t, err)
	body := decodeBody(t, response)
	require.Equal(t, true, body["configured"])
	require.Equal(t, false, body["connected"])
	require.Contains(t, body["message"], "rejected the access token")
	require.NotContains(t, string(response.Body), "super-secret-token")
}

func TestSourceControlActionsAreDelegated(t *testing.T) {
	t.Parallel()
	runtime := NewRuntime()
	runtime.SetHost(&configHost{config: map[string]any{}})

	// Delegation is proven by the recipe's own guard firing, rather than the
	// runtime's "unsupported action" path.
	_, err := runtime.HandleAction(context.Background(), &pluginsdk.PluginActionRequest{
		ActionKey: sourcecontrol.ActionRepositoriesList,
		Body:      []byte(`{}`),
	})
	require.ErrorContains(t, err, "requires a verified workspace")

	_, err = runtime.HandleAction(context.Background(), &pluginsdk.PluginActionRequest{
		ActionKey: "does.not.exist",
	})
	require.ErrorContains(t, err, "unsupported action")

	_, err = runtime.HandleAction(context.Background(), nil)
	require.ErrorContains(t, err, "action request is required")
}

func TestReferenceRPCsAreDelegatedAndFailClosed(t *testing.T) {
	t.Parallel()
	runtime := NewRuntime()
	runtime.SetHost(&configHost{config: map[string]any{}})

	// A search for a source this plugin does not own must be rejected.
	_, err := runtime.SearchEntityReferences(context.Background(), &pluginsdk.SearchEntityReferencesRequest{
		Source: "someone_elses_source", WorkspaceID: "workspace-1",
	})
	require.Error(t, err)

	authorized, err := runtime.AuthorizeEntityReference(context.Background(), &pluginsdk.AuthorizeEntityReferenceRequest{
		Source: ReferenceSource, WorkspaceID: "workspace-1", Purpose: "not-a-purpose",
	})
	require.NoError(t, err)
	require.False(t, authorized.Allowed)

	// Correct source and purpose, but the plugin is unconfigured: still denied.
	authorized, err = runtime.AuthorizeEntityReference(context.Background(), &pluginsdk.AuthorizeEntityReferenceRequest{
		Source: ReferenceSource, WorkspaceID: "workspace-1", Purpose: "submission",
		Reference: map[string]any{"id": "7:42"},
	})
	require.NoError(t, err)
	require.False(t, authorized.Allowed)
}

// The Host is injected after construction; nothing may panic before then.
func TestRuntimeToleratesMissingHost(t *testing.T) {
	t.Parallel()
	runtime := NewRuntime()
	response, err := runtime.HandleAction(context.Background(), &pluginsdk.PluginActionRequest{
		ActionKey: ActionConnectionGet,
	})
	require.NoError(t, err)
	require.Equal(t, false, decodeBody(t, response)["configured"])
}

func TestHandleActionHonorsCancellation(t *testing.T) {
	t.Parallel()
	runtime := NewRuntime()
	runtime.SetHost(&configHost{config: map[string]any{}})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := runtime.HandleAction(ctx, &pluginsdk.PluginActionRequest{
		ActionKey: sourcecontrol.ActionRepositoriesList,
	})
	require.ErrorIs(t, err, context.Canceled)
}

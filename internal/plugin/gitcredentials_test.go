package plugin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/kandev/kandev/pkg/pluginsdk"
	"github.com/stretchr/testify/require"
)

func userServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/user" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"login":"kandev"}`))
	}))
	t.Cleanup(server.Close)
	return server
}

func runtimeCredentialRequest(t *testing.T, server *httptest.Server, providerID string) *pluginsdk.ResolveGitCredentialRequest {
	t.Helper()
	parsed, err := url.Parse(server.URL)
	require.NoError(t, err)
	return &pluginsdk.ResolveGitCredentialRequest{
		ProviderID: providerID, WorkspaceID: "workspace-1", TaskID: "task-1", SessionID: "session-1",
		RepositoryID: "repository-1", Host: parsed.Host, Path: "/tools/widgets.git",
	}
}

func bindingFor(request *pluginsdk.ResolveGitCredentialRequest) *pluginsdk.GitCredentialBindingRequest {
	return &pluginsdk.GitCredentialBindingRequest{
		ProviderID: request.ProviderID, WorkspaceID: request.WorkspaceID, TaskID: request.TaskID,
		SessionID: request.SessionID, RepositoryID: request.RepositoryID, Host: request.Host, Path: request.Path,
	}
}

// Kandev only brokers generic-provider credentials through a plugin that
// implements both halves; a resolver without a binder fails closed.
func TestRuntimeIsGitCredentialHandler(t *testing.T) {
	t.Parallel()
	_, ok := any(NewRuntime()).(pluginsdk.GitCredentialHandler)
	require.True(t, ok)
}

func TestRuntimeResolvesGitCredentialForEnabledWorkspace(t *testing.T) {
	t.Parallel()
	server := userServer(t)
	runtime := NewRuntime()
	runtime.SetHost(newConfigHost(map[string]any{"base_url": server.URL, "api_token": "secret-token"}))
	handler, ok := any(runtime).(pluginsdk.GitCredentialHandler)
	require.True(t, ok)
	request := runtimeCredentialRequest(t, server, ProviderID)

	response, err := handler.ResolveGitCredential(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, "kandev", response.Username)
	require.Equal(t, "secret-token", response.Secret)

	binding, err := handler.GetGitCredentialBinding(context.Background(), bindingFor(request))
	require.NoError(t, err)
	require.NotEmpty(t, binding.Binding)
}

// The workspace toggle withdraws the integration, credentials included.
func TestDisabledWorkspaceWithholdsGitCredentials(t *testing.T) {
	t.Parallel()
	server := userServer(t)
	runtime := NewRuntime()
	runtime.SetHost(newConfigHost(map[string]any{"base_url": server.URL, "api_token": "secret-token"}))
	handler, ok := any(runtime).(pluginsdk.GitCredentialHandler)
	require.True(t, ok)
	setEnabled(t, runtime, "workspace-1", false)
	request := runtimeCredentialRequest(t, server, ProviderID)

	response, err := handler.ResolveGitCredential(context.Background(), request)
	require.Error(t, err)
	require.Nil(t, response)
	_, err = handler.GetGitCredentialBinding(context.Background(), bindingFor(request))
	require.Error(t, err)
}

func TestRuntimeRejectsGitCredentialForOtherProvider(t *testing.T) {
	t.Parallel()
	server := userServer(t)
	runtime := NewRuntime()
	runtime.SetHost(newConfigHost(map[string]any{"base_url": server.URL, "api_token": "secret-token"}))
	handler, ok := any(runtime).(pluginsdk.GitCredentialHandler)
	require.True(t, ok)
	request := runtimeCredentialRequest(t, server, "github")

	response, err := handler.ResolveGitCredential(context.Background(), request)
	require.Error(t, err)
	require.Nil(t, response)
	_, err = handler.GetGitCredentialBinding(context.Background(), bindingFor(request))
	require.Error(t, err)
}

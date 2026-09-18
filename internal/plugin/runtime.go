// Package plugin wires the concrete Forgejo adapters into the provider-neutral
// source-control extension and exposes the value pluginsdk.Serve runs.
package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"kandev-plugin-forgejo/internal/forgejo"
	"kandev-plugin-forgejo/internal/sourcecontrol"

	"github.com/kandev/kandev/pkg/pluginsdk"
)

const (
	// ProviderID is the repository provider this plugin owns. It must match
	// manifest repository_providers and the id the UI bundle registers.
	ProviderID = "forgejo"

	// ReferenceSource must match the manifest reference_sources entry exactly.
	ReferenceSource = "forgejo_pull_requests"

	// ActionConnectionGet reports connection status to the settings surface.
	ActionConnectionGet = "connection.get"
	// ActionConnectionTest validates the configured URL and token live.
	ActionConnectionTest = "connection.test"
)

// Runtime is the plugin value passed to pluginsdk.Serve. It embeds
// UnimplementedPlugin for no-op event/webhook defaults and delegates the
// source-control action, search, and authorize RPCs to the recipe extension.
type Runtime struct {
	pluginsdk.UnimplementedPlugin
	extension  *sourcecontrol.Extension
	connection *forgejo.Connection
}

var (
	_ pluginsdk.Plugin                    = (*Runtime)(nil)
	_ pluginsdk.ActionHandler             = (*Runtime)(nil)
	_ pluginsdk.EntityReferenceSearcher   = (*Runtime)(nil)
	_ pluginsdk.EntityReferenceAuthorizer = (*Runtime)(nil)
)

// NewRuntime builds the runtime and its adapter graph. The Host is injected
// after construction, so every adapter resolves it lazily through the provider
// closure rather than capturing it here.
func NewRuntime() *Runtime {
	runtime := &Runtime{}
	hosts := forgejo.HostProvider(func() pluginsdk.Host { return runtime.Host() })

	connection := forgejo.NewConnection(hosts)
	repositories := forgejo.NewRepositories(connection, hosts, ProviderID)
	associations := forgejo.NewAssociations(hosts)

	runtime.connection = connection
	runtime.extension = &sourcecontrol.Extension{
		ProviderID:           ProviderID,
		ReferenceSource:      ReferenceSource,
		Repositories:         repositories,
		RepositoryDetails:    repositories,
		AttachedRepositories: repositories,
		ChangeRequests:       forgejo.NewChangeRequests(connection, repositories),
		Associations:         associations,
		Reviews:              forgejo.NewReviews(connection, repositories, associations, ProviderID),
		References:           forgejo.NewReferences(connection),
	}
	return runtime
}

// HandleAction routes the connection actions this plugin owns and delegates
// every source-control action to the recipe extension.
func (r *Runtime) HandleAction(ctx context.Context, request *pluginsdk.PluginActionRequest) (*pluginsdk.PluginActionResponse, error) {
	if request == nil {
		return nil, errors.New("kandev-plugin-forgejo: action request is required")
	}
	switch request.ActionKey {
	case ActionConnectionGet:
		return r.connectionStatus(ctx, false)
	case ActionConnectionTest:
		return r.connectionStatus(ctx, true)
	default:
		return r.extension.HandleAction(ctx, request)
	}
}

// SearchEntityReferences delegates the composer `#` search.
func (r *Runtime) SearchEntityReferences(ctx context.Context, request *pluginsdk.SearchEntityReferencesRequest) (*pluginsdk.SearchEntityReferencesResponse, error) {
	return r.extension.SearchEntityReferences(ctx, request)
}

// AuthorizeEntityReference delegates live reference authorization.
func (r *Runtime) AuthorizeEntityReference(ctx context.Context, request *pluginsdk.AuthorizeEntityReferenceRequest) (*pluginsdk.AuthorizeEntityReferenceResponse, error) {
	return r.extension.AuthorizeEntityReference(ctx, request)
}

// connectionStatus reports whether the plugin is configured and, when probe is
// set, whether the instance accepts the configured token. The response never
// includes the token or any credential-bearing URL.
func (r *Runtime) connectionStatus(ctx context.Context, probe bool) (*pluginsdk.PluginActionResponse, error) {
	status := map[string]any{"provider": ProviderID, "configured": false, "connected": false}

	client, err := r.connection.Client(ctx)
	if err != nil {
		if errors.Is(err, forgejo.ErrNotConfigured) {
			status["message"] = "Set the instance URL and access token in Settings > Plugins > Forgejo."
			return jsonResponse(status)
		}
		status["message"] = safeMessage(err)
		return jsonResponse(status)
	}
	status["configured"] = true
	status["instance_url"] = client.Scope()
	status["host"] = client.Host()
	if !probe {
		return jsonResponse(status)
	}

	user, err := client.CurrentUser(ctx)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		status["message"] = safeMessage(err)
		return jsonResponse(status)
	}
	status["connected"] = true
	status["account"] = user.Login
	if version, err := client.Version(ctx); err == nil && strings.TrimSpace(version) != "" {
		status["instance_version"] = version
		status["flavor"] = string(client.DetectFlavor(ctx, version))
	}
	return jsonResponse(status)
}

// safeMessage maps an adapter error onto an operator-facing message. Provider
// error bodies are never forwarded: on some deployments they echo the token.
func safeMessage(err error) string {
	switch {
	case errors.Is(err, forgejo.ErrUnauthorized):
		return "The instance rejected the access token. Check that it is valid and has repository scope."
	case errors.Is(err, forgejo.ErrNotFound):
		return "The instance did not expose the expected REST v1 API at this URL."
	case errors.Is(err, forgejo.ErrNotConfigured):
		return "Set the instance URL and access token in Settings > Plugins > Forgejo."
	default:
		return "Could not reach the configured instance."
	}
}

func jsonResponse(value any) (*pluginsdk.PluginActionResponse, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("kandev-plugin-forgejo: encode action response: %w", err)
	}
	return &pluginsdk.PluginActionResponse{
		Body:    body,
		Headers: map[string]string{"Content-Type": "application/json"},
	}, nil
}

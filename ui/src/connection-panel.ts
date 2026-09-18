import type { Component, PluginHostApi } from "@kandev/plugin-sdk";

type ConnectionStatus = {
  configured?: unknown;
  connected?: unknown;
  instance_url?: unknown;
  instance_version?: unknown;
  flavor?: unknown;
  account?: unknown;
  message?: unknown;
};

function text(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}

/**
 * Turns any failure from the action bridge into something an operator can act
 * on. Host envelope errors ("workspace action requires only workspaceId") and
 * provider transport errors are implementation detail: they name the wrong
 * actor and read as though the operator typed something wrong. The raw error
 * still goes to the console so it stays diagnosable.
 */
function operatorMessage(cause: unknown): string {
  if (typeof console !== "undefined") {
    console.error("[kandev-plugin-forgejo] connection status request failed", cause);
  }
  return "Couldn't reach the Forgejo plugin. Check the Kandev server logs for details.";
}

/**
 * Connection status for the workspace integrations screen.
 *
 * The credentials themselves live in the manifest's `config_schema` and are
 * edited at Settings > Plugins > Forgejo; this panel never renders or collects
 * a token. It reports what the backend derived, and publishes the result to the
 * host so the per-workspace "Enabled" badge reflects it.
 */
export function createConnectionPanel(host: PluginHostApi): Component<{ workspaceId?: string }> {
  return function ForgejoConnectionPanel(props: { workspaceId?: string } = {}) {
    const [status, setStatus] = host.React.useState<ConnectionStatus | null>(null);
    const [checking, setChecking] = host.React.useState(false);
    const [error, setError] = host.React.useState<string | null>(null);

    // The host routes a workspace into this panel, but the same component can
    // be mounted without one; fall back to the active workspace and follow it.
    const [activeWorkspaceId, setActiveWorkspaceId] = host.React.useState<string | undefined>(
      () => host.context.getActiveWorkspaceId(),
    );
    host.React.useEffect(
      () => host.context.subscribeActiveWorkspace(setActiveWorkspaceId),
      [],
    );
    const workspaceId = text(props.workspaceId) || text(activeWorkspaceId) || "";

    const load = host.React.useCallback(
      async (probe: boolean, signal: AbortSignal) => {
        // Every action in the manifest is scope: "workspace". Calling one
        // without the selector is rejected by the host before it reaches the
        // plugin process, so there is nothing useful to request yet.
        if (!workspaceId) {
          setStatus(null);
          setError(null);
          setChecking(false);
          return;
        }
        setChecking(true);
        setError(null);
        try {
          const response = await host.api.invokeAction<ConnectionStatus>(
            probe ? "connection.test" : "connection.get",
            { workspaceId },
            { signal },
          );
          if (signal.aborted) return;
          setStatus(response);
          host.setIntegrationEnabled?.(
            "forgejo",
            workspaceId,
            response?.connected === true,
          );
        } catch (cause) {
          if (!signal.aborted) setError(operatorMessage(cause));
        } finally {
          if (!signal.aborted) setChecking(false);
        }
      },
      [workspaceId],
    );

    // The host gives this panel no AbortSignal, so it owns a controller and
    // aborts it on cleanup.
    host.React.useEffect(() => {
      const controller = new AbortController();
      void load(false, controller.signal);
      return () => controller.abort();
    }, [load]);

    const configured = status?.configured === true;
    const connected = status?.connected === true;

    let detail: string;
    if (!workspaceId) {
      detail = "Open a workspace to check the Forgejo connection.";
    } else if (connected) {
      const version = text(status?.instance_version);
      const flavor = text(status?.flavor);
      detail =
        `Connected to ${text(status?.instance_url)} as ${text(status?.account)}` +
        (version ? ` (${flavor || "instance"} ${version})` : "");
    } else {
      detail =
        text(status?.message) ||
        (configured ? "Not verified yet." : "Not configured.");
    }

    return host.jsx(
      "div",
      { className: "forgejo-connection" },
      host.jsx(
        "p",
        {
          className: "forgejo-connection__state",
          "data-state": connected ? "connected" : "disconnected",
        },
        detail,
      ),
      // The connection is plugin-wide: one instance URL and token serve every
      // workspace. Say so here rather than implying a per-workspace setting.
      host.jsx(
        "p",
        { className: "forgejo-connection__scope" },
        "This connection is shared by every workspace. Edit it at Settings > Plugins > Forgejo.",
      ),
      error ? host.jsx("p", { className: "forgejo-connection__error", role: "alert" }, error) : null,
      host.jsx(
        host.ui.Button,
        {
          type: "button",
          variant: "secondary",
          size: "sm",
          disabled: checking || !workspaceId,
          onClick: () => {
            const controller = new AbortController();
            void load(true, controller.signal);
          },
        },
        checking ? "Checking…" : "Test connection",
      ),
    );
  };
}

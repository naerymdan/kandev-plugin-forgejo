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
 * A small settings panel that reports whether the configured instance is
 * reachable. The credential itself lives in the manifest's `config_schema`, so
 * this panel never renders or collects a token — it only reports status the
 * backend derived.
 */
export function createConnectionPanel(host: PluginHostApi): Component {
  return function ForgejoConnectionPanel() {
    const [status, setStatus] = host.React.useState<ConnectionStatus | null>(null);
    const [checking, setChecking] = host.React.useState(false);
    const [error, setError] = host.React.useState<string | null>(null);

    const load = host.React.useCallback(
      async (probe: boolean, signal: AbortSignal) => {
        setChecking(true);
        setError(null);
        try {
          const response = await host.api.invokeAction<ConnectionStatus>(
            probe ? "connection.test" : "connection.get",
            {},
            { signal },
          );
          if (!signal.aborted) setStatus(response);
        } catch (cause) {
          if (!signal.aborted) {
            setError(cause instanceof Error ? cause.message : "Could not read connection status.");
          }
        } finally {
          if (!signal.aborted) setChecking(false);
        }
      },
      [],
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
    const detail = connected
      ? `Connected to ${text(status?.instance_url)} as ${text(status?.account)}` +
        (text(status?.instance_version) ? ` (${text(status?.flavor)} ${text(status?.instance_version)})` : "")
      : text(status?.message) || (configured ? "Not verified yet." : "Not configured.");

    return host.jsx(
      "div",
      { className: "forgejo-connection" },
      host.jsx(
        "p",
        { className: "forgejo-connection__state", "data-state": connected ? "connected" : "disconnected" },
        detail,
      ),
      error ? host.jsx("p", { className: "forgejo-connection__error", role: "alert" }, error) : null,
      host.jsx(
        host.ui.Button,
        {
          type: "button",
          variant: "secondary",
          size: "sm",
          disabled: checking,
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

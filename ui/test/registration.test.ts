import { readFileSync } from "node:fs";
import { beforeAll, describe, expect, it, vi } from "vitest";

type Lifecycle = {
  initialize(registry: Record<string, any>, host: Record<string, any>): void;
  destroy?(): void;
};

const registered: { id: string; lifecycle: Lifecycle }[] = [];

const registry = {
  registerRepositoryProvider: vi.fn(),
  registerTaskAction: vi.fn(),
  registerReviewProvider: vi.fn(),
  registerIntegrationSettings: vi.fn(),
};

const host = {
  jsx: vi.fn(() => null),
  React: { useState: vi.fn(() => [null, vi.fn()]), useEffect: vi.fn(), useCallback: (fn: unknown) => fn },
  ui: { Button: "button", ChangeRequestDetail: "detail" },
  api: { invokeAction: vi.fn() },
  openTaskLinkDialog: vi.fn(() => ({ close: vi.fn() })),
};

beforeAll(async () => {
  (globalThis as any).window = {
    registerKandevPlugin(id: string, lifecycle: Lifecycle) {
      registered.push({ id, lifecycle });
    },
  };
  await import("../src/bundle");
});

// manifest.yaml is the contract; the bundle must not drift from it.
function manifestValue(key: string): string {
  const manifest = readFileSync(new URL("../../manifest.yaml", import.meta.url), "utf8");
  const match = new RegExp(`^${key}:\\s*"?([^"\\n]+)"?`, "m").exec(manifest);
  return match?.[1]?.trim() ?? "";
}

describe("plugin registration", () => {
  it("registers under the manifest id", () => {
    expect(registered).toHaveLength(1);
    expect(registered[0]!.id).toBe("kandev-plugin-forgejo");
    expect(registered[0]!.id).toBe(manifestValue("id"));
  });

  it("registers every native source-control surface", () => {
    registered[0]!.lifecycle.initialize(registry as any, host as any);

    expect(registry.registerRepositoryProvider).toHaveBeenCalledTimes(1);
    expect(registry.registerTaskAction).toHaveBeenCalledTimes(1);
    expect(registry.registerReviewProvider).toHaveBeenCalledTimes(1);
    expect(registry.registerIntegrationSettings).toHaveBeenCalledTimes(1);

    const provider = registry.registerRepositoryProvider.mock.calls[0]![0];
    // Must equal manifest repository_providers[0] and the backend ProviderID.
    expect(provider.id).toBe("forgejo");
    expect(provider.label).toBe("Forgejo");
    expect(provider.supportsDraft).toBe(true);
    expect(typeof provider.listRepositories).toBe("function");
    expect(typeof provider.listBranches).toBe("function");
    expect(typeof provider.inspectURL).toBe("function");
    expect(typeof provider.createChangeRequest).toBe("function");

    const reviewProvider = registry.registerReviewProvider.mock.calls[0]![0];
    expect(reviewProvider.id).toBe("forgejo");
    expect(reviewProvider.changeRequestNoun).toBe("pull request");
    expect(typeof reviewProvider.unlink).toBe("function");
    expect(typeof reviewProvider.refreshAssociations).toBe("function");
    expect(typeof reviewProvider.ReviewPanel).toBe("function");

    const taskAction = registry.registerTaskAction.mock.calls[0]![0];
    expect(taskAction.placement).toBe("link");
    expect(taskAction.singleTaskOnly).toBe(true);
  });

  it("uses the action keys the manifest declares", async () => {
    const provider = registry.registerRepositoryProvider.mock.calls[0]![0];
    host.api.invokeAction.mockResolvedValue({ repositories: [] });
    await provider.listRepositories({
      workspaceId: "workspace-1",
      signal: new AbortController().signal,
    });
    expect(host.api.invokeAction).toHaveBeenCalledWith(
      "repositories.list",
      expect.objectContaining({ workspaceId: "workspace-1" }),
      expect.anything(),
    );
  });

  // Disable/enable runs the lifecycle again in the same tab.
  it("destroy is safe to call repeatedly", () => {
    const { destroy } = registered[0]!.lifecycle;
    expect(() => {
      destroy?.();
      destroy?.();
    }).not.toThrow();
  });
});

// ui/src/review-store.ts
function createSnapshotStore() {
  const snapshots = /* @__PURE__ */ new Map();
  const listeners = /* @__PURE__ */ new Map();
  const versions = /* @__PURE__ */ new Map();
  let epoch = 0;
  return {
    get: (key) => snapshots.get(key) ?? [],
    subscribe(key, listener) {
      const keyListeners = listeners.get(key) ?? /* @__PURE__ */ new Set();
      keyListeners.add(listener);
      listeners.set(key, keyListeners);
      return () => {
        keyListeners.delete(listener);
        if (keyListeners.size === 0) listeners.delete(key);
      };
    },
    beginRefresh(key) {
      const version = (versions.get(key) ?? 0) + 1;
      versions.set(key, version);
      return { epoch, version };
    },
    commit(key, token, values) {
      if (token.epoch !== epoch || versions.get(key) !== token.version) return false;
      snapshots.set(key, [...values]);
      listeners.get(key)?.forEach((listener) => listener());
      return true;
    },
    clear() {
      epoch += 1;
      versions.clear();
      snapshots.clear();
      listeners.forEach((keyListeners) => keyListeners.forEach((listener) => listener()));
      listeners.clear();
    }
  };
}

// ui/src/source-control.ts
function record(value) {
  return value !== null && typeof value === "object" ? value : {};
}
function text(value) {
  return typeof value === "string" ? value.trim() : "";
}
function finiteNumber(value) {
  return typeof value === "number" && Number.isFinite(value) ? value : void 0;
}
function nonNegativeInteger(value) {
  const number = finiteNumber(value);
  return number !== void 0 && Number.isInteger(number) && number >= 0 ? number : void 0;
}
function positiveInteger(value) {
  const number = nonNegativeInteger(value);
  return number !== void 0 && number > 0 ? number : void 0;
}
function normalizeTaskStatus(value) {
  const source = record(value);
  const number = positiveInteger(source.number);
  const state = text(source.state);
  const pipelineState = text(source.pipeline_state);
  if (number === void 0 || !["open", "merged", "closed", "draft"].includes(
    state
  ) || !["success", "failure", "pending", "neutral"].includes(
    pipelineState
  )) {
    return void 0;
  }
  const checks = Array.isArray(source.checks) ? source.checks.flatMap((value2) => {
    const check = record(value2);
    const id = text(check.id);
    const label = text(check.label);
    const checkState = text(check.state);
    if (!id || !label || !["success", "failure", "pending", "neutral"].includes(
      checkState
    )) {
      return [];
    }
    const detail = text(check.detail);
    const url = text(check.url);
    return [{
      id,
      label,
      state: checkState,
      ...detail ? { detail } : {},
      ...url ? { url } : {}
    }];
  }) : [];
  const reviewSource = record(source.review);
  const reviewState = text(reviewSource.state);
  const approved = nonNegativeInteger(reviewSource.approved);
  const required = nonNegativeInteger(reviewSource.required);
  const requested = nonNegativeInteger(reviewSource.requested);
  const review = approved !== void 0 && ["approved", "changes_requested", "pending"].includes(
    reviewState
  ) ? {
    state: reviewState,
    approved,
    ...required === void 0 ? {} : { required },
    ...requested === void 0 ? {} : { requested }
  } : void 0;
  const unresolvedComments = nonNegativeInteger(source.unresolved_comments);
  const updatedAt = finiteNumber(source.updated_at);
  return {
    number,
    state,
    pipelineState,
    checks,
    ...review ? { review } : {},
    ...unresolvedComments === void 0 ? {} : { unresolvedComments },
    ...updatedAt === void 0 ? {} : { updatedAt }
  };
}
function normalizeReview(providerId, value) {
  const source = record(value);
  const reviewKey = text(source.review_key);
  const title = text(source.title);
  const url = text(source.url);
  const connectionScope = text(source.connection_scope);
  const repositoryId = text(source.repository_id);
  const changeRequestNumber = positiveInteger(source.change_request_number);
  if (!reviewKey || !title || !url || !connectionScope || !repositoryId || changeRequestNumber === void 0) {
    return null;
  }
  const state = text(source.state);
  const taskStatus = normalizeTaskStatus(source.task_status);
  return {
    providerId,
    reviewKey,
    title,
    url,
    connectionScope,
    repositoryId,
    changeRequestNumber,
    state,
    ...taskStatus ? { taskStatus } : {}
  };
}
function normalizeAssociation(providerId, value) {
  const source = record(value);
  const taskId = text(source.task_id);
  const reviewKey = text(source.review_key);
  const connectionScope = text(source.connection_scope);
  const repositoryId = text(source.repository_id);
  const changeRequestNumber = positiveInteger(source.change_request_number);
  if (!taskId || !reviewKey || !connectionScope || !repositoryId || changeRequestNumber === void 0) {
    return null;
  }
  return {
    providerId,
    taskId,
    reviewKey,
    connectionScope,
    repositoryId,
    changeRequestNumber
  };
}
function normalizeRepository(providerId, value) {
  const source = record(value);
  const providerHost = text(source.provider_host);
  const ownerOrProject = text(source.owner_or_project);
  const repositoryId = text(source.repository_id);
  const repositoryName = text(source.name);
  const cloneUrl = text(source.clone_url);
  if (!providerHost || !ownerOrProject || !repositoryId || !repositoryName || !cloneUrl) return null;
  const providerScope = text(source.provider_scope);
  const defaultBranch = text(source.default_branch);
  return {
    providerId,
    providerHost,
    ...providerScope ? { providerScope } : {},
    ownerOrProject,
    repositoryId,
    repositoryName,
    cloneUrl,
    ...defaultBranch ? { defaultBranch } : {}
  };
}
function credentialFreeRepository(repository) {
  return {
    provider_id: repository.providerId,
    provider_host: repository.providerHost,
    provider_scope: repository.providerScope ?? "",
    provider_repository_id: repository.repositoryId,
    owner_or_project: repository.ownerOrProject,
    name: repository.repositoryName,
    clone_url: repository.cloneUrl,
    default_branch: repository.defaultBranch ?? ""
  };
}
function registerSourceControlRecipe(registry, host, options) {
  const reviewStore = createSnapshotStore();
  const associationStore = createSnapshotStore();
  const overlays = /* @__PURE__ */ new Set();
  async function refreshReviews(taskId, signal, workspaceId) {
    const token = reviewStore.beginRefresh(taskId);
    signal.throwIfAborted();
    const response = await host.api.invokeAction(
      "change_requests.get",
      { ...workspaceId ? { workspaceId } : {}, taskId },
      { signal }
    );
    signal.throwIfAborted();
    reviewStore.commit(
      taskId,
      token,
      (response.reviews ?? []).flatMap((review) => {
        const normalized = normalizeReview(options.providerId, review);
        return normalized ? [normalized] : [];
      })
    );
  }
  async function refreshAssociations(workspaceId, signal) {
    const token = associationStore.beginRefresh(workspaceId);
    signal.throwIfAborted();
    const response = await host.api.invokeAction(
      "change_requests.associations",
      { workspaceId },
      { signal }
    );
    signal.throwIfAborted();
    associationStore.commit(
      workspaceId,
      token,
      (response.associations ?? []).flatMap((association) => {
        const normalized = normalizeAssociation(options.providerId, association);
        return normalized ? [normalized] : [];
      })
    );
  }
  async function refreshAfterMutation(workspaceId, taskId, signal) {
    try {
      await Promise.all([
        refreshReviews(taskId, signal, workspaceId),
        refreshAssociations(workspaceId, signal)
      ]);
    } catch {
    }
  }
  const repositoryProvider = {
    id: options.providerId,
    label: options.label,
    ...options.icon ? { icon: options.icon } : {},
    ...options.matchesURL ? { matchesURL: options.matchesURL } : {},
    ...options.supportsDraft === void 0 ? {} : { supportsDraft: options.supportsDraft },
    async listRepositories({ workspaceId, query = "", cursor = "", limit = 100, signal }) {
      signal.throwIfAborted();
      const response = await host.api.invokeAction(
        "repositories.list",
        { workspaceId, body: { query, cursor, limit } },
        { signal }
      );
      signal.throwIfAborted();
      const nextCursor = text(response.next_cursor);
      return {
        repositories: (response.repositories ?? []).flatMap((repository) => {
          const normalized = normalizeRepository(options.providerId, repository);
          return normalized ? [normalized] : [];
        }),
        // Omit the key entirely rather than setting it to undefined: the host
        // types it as an optional string under exactOptionalPropertyTypes.
        ...nextCursor ? { nextCursor } : {}
      };
    },
    async listBranches({ workspaceId, repository, signal }) {
      signal.throwIfAborted();
      const response = await host.api.invokeAction(
        "repositories.branches",
        { workspaceId, body: { repository: credentialFreeRepository(repository) } },
        { signal }
      );
      signal.throwIfAborted();
      return (response.branches ?? []).flatMap((branch) => {
        const name = text(record(branch).name);
        return name ? [{ name }] : [];
      });
    },
    async inspectURL({ workspaceId, url, signal }) {
      signal.throwIfAborted();
      const response = await host.api.invokeAction(
        "repositories.inspect",
        { workspaceId, body: { url } },
        { signal }
      );
      signal.throwIfAborted();
      return normalizeRepository(options.providerId, response.repository);
    },
    async createChangeRequest({
      workspaceId,
      taskId,
      sessionId,
      repositoryId,
      title,
      body,
      baseBranch,
      draft,
      signal
    }) {
      signal.throwIfAborted();
      const response = await host.api.invokeAction(
        "change_requests.create",
        {
          workspaceId,
          taskId,
          sessionId,
          repositoryId,
          body: {
            title,
            description: body,
            destination: baseBranch ?? "",
            draft
          }
        },
        { signal }
      );
      const url = text(response.url);
      if (!url) throw new Error("source-control recipe: create response did not include a URL");
      await refreshAfterMutation(workspaceId, taskId, signal);
      const output = text(response.output);
      const associationError = text(response.association_error);
      return {
        url,
        provider: options.providerId,
        ...output ? { output } : {},
        ...typeof response.linked === "boolean" ? { linked: response.linked } : {},
        ...associationError ? { associationError } : {}
      };
    }
  };
  registry.registerRepositoryProvider(repositoryProvider);
  registry.registerTaskAction({
    id: `${options.providerId}-link-change-request`,
    label: `${options.label} ${options.changeRequestNoun}`,
    ...options.icon ? { icon: options.icon } : {},
    placement: "link",
    singleTaskOnly: true,
    async run(context) {
      const dialog = host.openTaskLinkDialog({
        title: `Link ${options.label} ${options.changeRequestNoun}`,
        description: `Enter a ${options.label} ${options.changeRequestNoun} URL or canonical reference.`,
        inputLabel: options.changeRequestNoun,
        emptyError: `Enter a valid ${options.label} ${options.changeRequestNoun} reference.`,
        failureMessage: `Failed to link ${options.label} ${options.changeRequestNoun}.`,
        successMessage: `${options.label} ${options.changeRequestNoun} linked`,
        inputTestId: `${options.providerId}-review-reference`,
        errorTestId: `${options.providerId}-review-reference-error`,
        submitTestId: `${options.providerId}-review-reference-submit`,
        async onSubmit(reference, signal) {
          signal.throwIfAborted();
          const parsed = options.parseReference(reference);
          if (!parsed) {
            throw new Error(`Enter a valid ${options.label} ${options.changeRequestNoun} reference.`);
          }
          await host.api.invokeAction(
            "change_requests.link",
            {
              workspaceId: context.workspaceId,
              taskId: context.taskId,
              body: { reference: parsed }
            },
            { signal }
          );
          await refreshAfterMutation(context.workspaceId, context.taskId, signal);
        }
      });
      overlays.add(dialog);
    }
  });
  registry.registerReviewProvider({
    id: options.providerId,
    label: options.label,
    ...options.icon ? { icon: options.icon } : {},
    changeRequestNoun: options.changeRequestNoun,
    order: options.order ?? 100,
    getSnapshot: (taskId) => reviewStore.get(taskId),
    subscribe: (taskId, listener) => reviewStore.subscribe(taskId, listener),
    refresh: (taskId, signal) => refreshReviews(taskId, signal),
    getAssociationSnapshot: (workspaceId) => associationStore.get(workspaceId),
    subscribeAssociations: (workspaceId, listener) => associationStore.subscribe(workspaceId, listener),
    refreshAssociations,
    async unlink({
      workspaceId,
      taskId,
      connectionScope,
      repositoryId,
      changeRequestNumber,
      signal
    }) {
      const number = typeof changeRequestNumber === "string" && /^\d+$/.test(changeRequestNumber) ? positiveInteger(Number(changeRequestNumber)) : positiveInteger(changeRequestNumber);
      if (!connectionScope.trim() || !repositoryId.trim() || number === void 0) {
        throw new Error("source-control recipe: cannot unlink an incomplete review identity");
      }
      signal.throwIfAborted();
      await host.api.invokeAction(
        "change_requests.unlink",
        {
          workspaceId,
          taskId,
          body: {
            connection_scope: connectionScope,
            repository_id: repositoryId,
            number
          }
        },
        { signal }
      );
    },
    ReviewPanel: (props) => {
      const review = reviewStore.get(props.taskId).find(
        (candidate) => candidate.reviewKey === props.reviewKey && candidate.connectionScope === props.connectionScope && candidate.repositoryId === props.repositoryId && String(candidate.changeRequestNumber) === String(props.changeRequestNumber)
      );
      return host.jsx(host.ui.ChangeRequestDetail, {
        detail: review ? options.toChangeRequestDetail(review) : null,
        presentation: props.presentation,
        loading: false,
        error: null
      });
    }
  });
  return {
    destroy() {
      overlays.forEach((overlay) => overlay.close());
      overlays.clear();
      reviewStore.clear();
      associationStore.clear();
    }
  };
}

// ui/src/forgejo-icon.ts
function createForgejoIcon(host) {
  return function ForgejoIcon({ className } = {}) {
    return host.jsx(
      "svg",
      {
        className,
        viewBox: "0 0 24 24",
        width: "1em",
        height: "1em",
        fill: "none",
        stroke: "currentColor",
        strokeWidth: 2,
        strokeLinecap: "round",
        strokeLinejoin: "round",
        "aria-hidden": "true",
        focusable: "false"
      },
      host.jsx("circle", { cx: 6, cy: 5, r: 2.5 }),
      host.jsx("circle", { cx: 18, cy: 5, r: 2.5 }),
      host.jsx("circle", { cx: 6, cy: 19, r: 2.5 }),
      host.jsx("path", { d: "M6 7.5v9" }),
      host.jsx("path", { d: "M18 7.5v2a4 4 0 0 1-4 4h-4" })
    );
  };
}

// ui/src/connection-panel.ts
function text2(value) {
  return typeof value === "string" ? value.trim() : "";
}
function createConnectionPanel(host) {
  return function ForgejoConnectionPanel() {
    const [status, setStatus] = host.React.useState(null);
    const [checking, setChecking] = host.React.useState(false);
    const [error, setError] = host.React.useState(null);
    const load = host.React.useCallback(
      async (probe, signal) => {
        setChecking(true);
        setError(null);
        try {
          const response = await host.api.invokeAction(
            probe ? "connection.test" : "connection.get",
            {},
            { signal }
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
      []
    );
    host.React.useEffect(() => {
      const controller = new AbortController();
      void load(false, controller.signal);
      return () => controller.abort();
    }, [load]);
    const configured = status?.configured === true;
    const connected = status?.connected === true;
    const detail = connected ? `Connected to ${text2(status?.instance_url)} as ${text2(status?.account)}` + (text2(status?.instance_version) ? ` (${text2(status?.flavor)} ${text2(status?.instance_version)})` : "") : text2(status?.message) || (configured ? "Not verified yet." : "Not configured.");
    return host.jsx(
      "div",
      { className: "forgejo-connection" },
      host.jsx(
        "p",
        { className: "forgejo-connection__state", "data-state": connected ? "connected" : "disconnected" },
        detail
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
          }
        },
        checking ? "Checking\u2026" : "Test connection"
      )
    );
  };
}

// ui/src/detail.ts
function toChangeRequestDetail(review) {
  const status = review.taskStatus;
  return {
    provider: "forgejo",
    providerLabel: "Forgejo",
    number: status?.number ?? review.changeRequestNumber,
    title: review.title,
    url: review.url,
    state: status?.state ?? review.state,
    pipelineState: status?.pipelineState ?? "neutral",
    checks: (status?.checks ?? []).map((check) => ({
      id: check.id,
      label: check.label,
      state: check.state,
      ...check.detail ? { detail: check.detail } : {},
      ...check.url ? { url: check.url } : {}
    })),
    ...status?.review ? {
      review: {
        state: status.review.state,
        approved: status.review.approved,
        ...status.review.required === void 0 ? {} : { required: status.review.required },
        ...status.review.requested === void 0 ? {} : { requested: status.review.requested }
      }
    } : {},
    ...status?.unresolvedComments === void 0 ? {} : { unresolvedComments: status.unresolvedComments },
    ...status?.updatedAt === void 0 ? {} : { updatedAt: status.updatedAt }
  };
}

// ui/src/references.ts
var OWNER_REPO_NUMBER = /^[^/\s]+\/[^#\s]+#[1-9]\d*$/;
var PULL_REQUEST_PATH = /\/([^/\s]+)\/([^/\s]+)\/pulls\/([1-9]\d*)(?:[/?#].*)?$/;
function parsePullRequestReference(reference) {
  const trimmed = reference.trim();
  if (!trimmed) return null;
  if (OWNER_REPO_NUMBER.test(trimmed)) return trimmed;
  if (!trimmed.includes("://")) return null;
  let url;
  try {
    url = new URL(trimmed);
  } catch {
    return null;
  }
  const match = PULL_REQUEST_PATH.exec(url.pathname);
  if (!match) return null;
  const [, owner, repository, number] = match;
  if (!owner || !repository || !number || Number(number) <= 0) return null;
  return trimmed;
}
function looksLikeRepositoryURL(url) {
  const trimmed = url.trim();
  if (!trimmed) return false;
  if (/^[\w.+-]+@[^:\s]+:[^\s]+$/.test(trimmed)) return true;
  try {
    const parsed = new URL(trimmed);
    return parsed.protocol === "http:" || parsed.protocol === "https:";
  } catch {
    return false;
  }
}

// ui/src/bundle.ts
var PLUGIN_ID = "kandev-plugin-forgejo";
var PROVIDER_ID = "forgejo";
var sourceControl;
window.registerKandevPlugin(PLUGIN_ID, {
  initialize(registry, host) {
    const icon = createForgejoIcon(host);
    sourceControl = registerSourceControlRecipe(registry, host, {
      providerId: PROVIDER_ID,
      label: "Forgejo",
      icon,
      changeRequestNoun: "pull request",
      order: 40,
      supportsDraft: true,
      matchesURL: looksLikeRepositoryURL,
      parseReference: parsePullRequestReference,
      toChangeRequestDetail
    });
    registry.registerIntegrationSettings({
      id: PROVIDER_ID,
      label: "Forgejo",
      description: "Connect a Forgejo or Gitea instance for repositories, pull requests, and reviews.",
      icon,
      Component: createConnectionPanel(host)
    });
  },
  // initialize may run again in the same tab after a disable/enable cycle, so
  // destroy must leave no timers, listeners, or cached snapshots behind.
  destroy() {
    sourceControl?.destroy();
    sourceControl = void 0;
  }
});

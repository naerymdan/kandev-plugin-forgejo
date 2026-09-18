# Changelog

## 0.1.1

- The workspace integrations panel invoked its actions without a `workspaceId`.
  Every action is `scope: "workspace"`, so Kandev rejected each call at the
  envelope before it reached the plugin process; the panel then showed "not
  configured" and surfaced the raw host error, against a backend that was
  working. It now uses the workspace the host routes to it, falls back to the
  active workspace, and makes no call when neither resolves.
- `connection.get` always reported `connected: false` because it never probed,
  so the panel read as disconnected on every mount even right after a
  successful **Test connection**. It now serves a probe result cached per
  workspace for 60s and probes when that is stale. The cache is bound to the
  configured instance URL and dropped when a probe fails.
- Host and transport errors are no longer shown verbatim. The panel renders one
  actionable sentence and logs the cause to the console.
- Publishes the integration's enabled state per workspace via
  `host.setIntegrationEnabled`, which drives Kandev's enabled badge.
- `registerIntegrationSettings` is behind a capability check, so a host without
  that hook cannot abort `initialize` and lose the source-control registrations.
- Release binaries are built with `-trimpath -ldflags="-s -w"`: the package drops
  from 50.6 MB to 27.8 MB, and binaries no longer embed the build machine's
  filesystem paths. Panic traces keep function names and line numbers.
- Documents the HTTP install/config/action surface for headless setups, and
  states explicitly that one connection is shared by every workspace.

## 0.1.0

Initial release.

- Forgejo/Gitea repository provider: server-side search, paging, branch lists,
  and authenticated URL ownership checks.
- Pull-request creation from a task's verified worktree branch, with draft
  expressed as a portable `WIP:` title marker.
- Task **Link** action accepting a pull-request URL or `owner/repo#number`,
  resolved server-side before it is stored.
- Review snapshots with semantic state, commit-status checks, approval counts,
  and unresolved review comments, plus a workspace-level association map.
- Composer `#` pull-request references with fail-closed submit-time
  authorization.
- Distinguishes Forgejo from Gitea by probing Forgejo's `/api/forgejo/v1`
  namespace, so the label survives Forgejo dropping its `+gitea-` version
  suffix. Display only; no behavior branches on it.
- Requires a token with `read:repository`, `read:user` and `read:issue`, plus
  `write:repository` to open pull requests. Without `read:user` the connection
  test reports "not connected"; without `read:issue` the composer `#` picker
  returns nothing.
- Declares a supported floor of Gitea 1.20 / Forgejo 7.0. The floor is a
  token-scope boundary, not a capability one: Gitea ≤1.18 has no scope system
  (tokens carry full account access), and 1.19 uses an incompatible vocabulary
  whose API-minted tokens cannot write.
- Verified with the full live contract suite against 14 Gitea releases
  (1.14.7 through 1.27.3) and 10 Forgejo releases (7.0.16 through 16.0.5).
  All 24 returned complete data.

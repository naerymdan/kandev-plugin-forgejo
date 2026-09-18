# Changelog

## 0.2.0

Agent-facing MCP tools. Task agents can now drive Forgejo directly instead of
being handed `curl` recipes in a workflow step prompt.

- Adds three tools on the `kanban-task` surface: `ci` (CI result for a branch or
  commit, with a log id per job), `ci_log` (the tail of one job's log), and `pr`
  (get, open, or ready the task's pull request).
- `pr op=open` is idempotent rather than retried. Kandev never retries an agent
  tool, because it cannot know whether the side effect already landed, so open
  returns an existing pull request for the same head instead of failing — and
  checks again if the create itself errors.
- `pr op=open` records the Kandev task association, so a pull request an agent
  opened appears in the review sidebar exactly like one opened from the UI.
- `pr op=ready` clears the `WIP:` title prefix. REST v1 has no draft flag on
  either host, so that prefix is the draft and a title edit is the way out of it.
- `ci` resolves through `/actions/runs`, then `/actions/tasks`, then the
  combined commit status, taking the first that answers. The endpoints differ
  by release far more than the rest of the REST v1 surface does: Gitea 1.20 has
  none of them, Gitea gained `/actions/runs` after 1.24, and Forgejo 13 lists
  runs but serves neither their jobs nor job logs. The commit status is present
  everywhere and is also the only surface that sees CI running outside the
  forge, which on self-hosted Forgejo is common.
- A ref with no CI reports `none`, never `failure`. "Nothing ran" and "something
  broke" are not the same answer to give an agent.
- Job logs are streamed through a tail buffer and requested with a suffix
  `Range`, so a long log is never materialized and a host that ignores `Range`
  still yields the end of the log rather than the start.
- Normalizes one measured host difference: Gitea answers an unknown job id with
  HTTP 500 where Forgejo answers 404 (1.24.7 against 16.0.5). Reporting that as
  a server fault would send an agent chasing an outage instead of a stale id.
- The tools honor the per-workspace enable switch and make no request to the
  instance while it is off.
- `min_kandev_version` moves to **0.95.0**, where plugin tools are served over
  Kandev's MCP endpoint. An older host ignores an unknown manifest block rather
  than refusing the install, so leaving the floor at 0.88.0 would have installed
  a plugin whose tools silently never appeared. A host on 0.88–0.94 can still
  run release 0.1.2 for the source-control surfaces.
- The three tool definitions cost 419 tokens together, measured with Kandev's
  own estimator; a test holds a ceiling on the set so a fourth has to be argued
  for.

## 0.1.2

- The integrations card now renders its own enable/disable switch. Kandev does
  not supply one for a plugin integration, which is why this card had no toggle
  while native integrations did.
- The switch is honored, not decorative. The choice is stored per workspace and
  the backend enforces it: while off, repository, branch, review and association
  reads return nothing and pull-request create/link/unlink refuse. A workspace
  with no stored choice is enabled.
- Adds the `connection.set_enabled` action, and `connection.get` now reports
  `enabled` alongside connection state.

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

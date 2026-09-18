# kandev-plugin-forgejo

Connect a self-hosted [Forgejo](https://forgejo.org/) or [Gitea](https://about.gitea.com/)
instance to Kandev's **native** repository, pull-request, task-link and review
surfaces.

This is a Kandev runtime plugin. It adds no screens of its own beyond a small
connection panel: it supplies provider data through the source-control
extension contracts, and Kandev renders it with the same UI it uses for the
built-in code hosts.

## What it adds

| Surface | What you get |
| --- | --- |
| Remote repository picker | Forgejo repositories appear alongside the built-in providers, with server-side search, paging, and branch lists. |
| Create PR dialog | Opens a pull request from the task's verified worktree branch. Draft is honored with the portable `WIP:` title marker. |
| Task **Link** menu | A "Forgejo pull request" entry that accepts a pull-request URL or `owner/repo#number`. |
| Sidebar / Kanban / list glyphs | Pull-request status per task, from one workspace-level association map. |
| Integrations card | Connection status and a per-workspace enable switch the plugin renders itself. |
| Review panel + CI popover | Review state, approval counts, individual commit statuses, and unresolved review comments, on desktop and mobile. |
| Composer `#` references | Search pull requests from the composer; access is re-checked live at submit time. |
| Agent tools (MCP) | Two tools on task sessions — read CI for a ref (optionally with each failing job's log) and get/open/ready the task's pull request. |

## Requirements

- Kandev **0.95.0** or newer, which is where plugin `agent_tools` are served over
  Kandev's MCP endpoint. The source-control half alone needs only `v0.88.0`, and
  a `0.88`–`0.94` host will happily run release `0.1.2`; it would also install
  `0.2.0` and silently never show the agent tools, which is why the floor moved
  rather than staying put.
- **Gitea 1.20+** or **Forgejo 7.0+**, reachable from the Kandev backend and
  serving the REST v1 API at `<instance URL>/api/v1`. See "Supported versions"
  for what was tested and why the floor sits there.
- A personal access token. Gitea 1.20+ and every supported Forgejo enforce token
  scopes, and the plugin needs all four of:

  | Scope | Needed for |
  | --- | --- |
  | `read:repository` | Repository search, branches, pull requests, reviews, commit statuses |
  | `write:repository` | Opening pull requests (omit for a read-only install) |
  | `read:user` | The connection test and the account shown in settings |
  | `read:issue` | Composer `#` pull-request search |

  A token missing `read:user` reports "not connected" even though everything
  else works, and one missing `read:issue` returns an empty composer picker.
  On Gitea 1.14–1.19 these scope names do not exist; see "Supported versions".

## Install

1. Download `kandev-plugin-forgejo-<version>.tar.gz` from
   [Releases](https://github.com/naerymdan/kandev-plugin-forgejo/releases).
2. In Kandev: **Settings → Plugins → Install from file**.
3. Open **Settings → Plugins → Forgejo** and set:
   - **Instance URL** — e.g. `https://codeberg.org` or `http://forge.lan:3000`.
   - **Access token** — with the scopes above. Stored in Kandev's encrypted
     vault and readable only inside the plugin process.
4. Use **Test connection** in **Settings → Integrations → Forgejo** to confirm
   the instance is reachable.

Changing the configuration restarts the plugin; that is expected.

## Forgejo and Gitea

Forgejo is a hard fork of Gitea. Forgejo has diverged substantially in its web
UI and features, but it still serves the Gitea-compatible REST surface at
`/api/v1` and continues to declare its compatibility level in the version
string (`16.0.5+gitea-1.22.0`). This plugin deliberately restricts itself to
endpoints and fields that exist on both, so one plugin serves either host.

Every request/response shape this plugin depends on was compared field by field
across Forgejo 13.0.5, Forgejo 16.0.5, and Gitea 1.24.7. They are identical;
the only difference is the version string itself.

The connection panel labels which flavor it detected, but **no behavior
branches on it**. Detection asks for Forgejo's own `/api/forgejo/v1/version`
namespace, which Gitea does not serve, rather than matching on the
`+gitea-<compat>` suffix — that suffix is a compatibility declaration Forgejo
could stop publishing as it diverges further, and the namespace probe keeps
working if it does.

Two places where the shared surface differs from what a GitHub-shaped client
would assume, and which this plugin handles explicitly:

- A combined commit status uses `state`, but each entry inside it uses
  `status`. Reading `state` on the entries yields no checks at all.
- REST v1 has **no draft flag** when creating a pull request. Draft is expressed
  with a `WIP:` title prefix, which both web UIs recognize; an existing marker
  is not doubled.

### Supported versions

**Floor: Gitea 1.20, Forgejo 7.0.** Newer is always fine — the current releases
of both are covered below.

That floor is a token-scope boundary, not a capability one. Every version in the
table works; below the floor the *setup instructions differ*, which is the part
that cannot be documented once and stay true:

| Gitea range | Token behavior |
| --- | --- |
| ≤ 1.18 | No scope system at all. The `scopes` field is ignored and a token has **full account access** — strictly worse for an integration credential. |
| 1.19 | Scopes exist but use an incompatible vocabulary: `read:repository` is rejected outright, and a token minted through the API comes back with `scopes: null` and is then **denied every write**. |
| ≥ 1.20 | The scope names in "Requirements" above are accepted and enforced. A read-only token really is read-only. |

### Verified against

Every version below was exercised with the full live contract suite — repository
discovery, review snapshot with CI status and approvals, composer reference
search and authorization, and pull-request creation — and all of them returned
complete data, not a degraded fallback.

| Host | Versions verified | Result |
| --- | --- | --- |
| Gitea | 1.14.7, 1.15.11, 1.16.9, 1.17.4, 1.18.5, 1.19.4, 1.20.6, 1.21.11, 1.22.6, 1.23.8, 1.24.7, 1.25.5, 1.26.4, 1.27.3 | 14/14 fully working |
| Forgejo | 7.0.16, 8.0.3, 9.0.3, 10.0.3, 11.0.16, 12.0.4, 13.0.5, 14.0.5, 15.0.9, 16.0.5 | 10/10 fully working |

Gitea 1.14.7 is from April 2021, so the shared REST v1 surface this plugin uses
has been stable for over five years. Gitea 1.14–1.19 are therefore *known to
work* but **not supported**: they are end-of-life upstream and need
version-specific token instructions.

Forgejo 7.0 is the oldest Forgejo tested and is the oldest release Forgejo
itself still supports. Older Forgejo (the v1.x line) is untested.

CI runs the same suite against the floor and the current release of each host on
every change, and asserts the detected flavor matches the host under test. If
the shared surface ever diverges, that matrix fails first.

## Agent tools

Task agents reach Forgejo through two MCP tools, exposed on the `kanban-task`
surface as `kandev_kandev_plugin_forgejo_ci` and `…_pr`. They exist to replace
hand-written `curl` recipes in workflow step prompts — a PR step that had to
scrape a token out of `~/.git-credentials` and hand-roll `/actions/runs`
filtering can ask for `ci` instead.

| Tool | Arguments | Answers |
| --- | --- | --- |
| `ci` | `ref` (branch or SHA), `logs?` (tail lines per failed job), `repo?` | Overall state (`success`/`failure`/`running`/`pending`/`none`) and each job, with the tail of each failed job's log when `logs` is set. |
| `pr` | `op` = `get` / `open` / `ready`, plus `head`, `base`, `title`, `body`, `draft`, `repo` | The task's pull request. `open` records the Kandev task association, so the review sidebar sees it too. |

A few properties worth knowing before you write a prompt against them:

- **`open` is idempotent.** Kandev never retries an agent tool — it cannot know
  whether a side effect already landed — so `open` checks for an existing open
  pull request on the same head first, and checks again if the create fails.
  Calling it twice returns the same pull request, marked `already open`.
- **`ready` is the draft flag Forgejo does not have.** REST v1 exposes no draft
  boolean on either host, so a draft is a `WIP:` title prefix and `ready` is a
  title edit. Readying an already-ready pull request sends nothing.
- **`repo` is only needed for a task with more than one Forgejo repository.**
  With several attached, the tools refuse rather than guess, and name them.
- **`logs` is opt-in and bounded.** Without it `ci` fetches no logs at all. With
  it, the tails of at most five failed jobs are inlined under a shared 256 KiB
  budget, and a job whose log the release does not serve says so in place of
  its own section rather than failing the read.
- **They respect the workspace toggle.** With the integration switched off the
  tools return an error and make no request to the instance.
- **A ref with no CI reports `none`, not failure.** An agent must not read
  "nothing ran" as "something broke".

### What CI data you get, by version

`ci` resolves through three surfaces and takes the first that answers, because
this is where Forgejo and Gitea have genuinely diverged. Measured directly:

| Endpoint | Gitea 1.20 | Gitea 1.24 | Gitea 1.27 | Forgejo 7 | Forgejo 13 | Forgejo 16 |
| --- | --- | --- | --- | --- | --- | --- |
| `/actions/runs` | — | — | yes | — | yes | yes |
| `/actions/runs/{id}/jobs` | — | — | yes | — | — | yes |
| `/actions/tasks` | — | yes | yes | — | yes | yes |
| `/actions/jobs/{id}/logs` | — | yes | yes | — | — | yes |
| `/commits/{ref}/status` | yes | yes | yes | yes | yes | yes |

Read from each release's own `swagger.v1.json` and confirmed against running
instances. At the supported floor of either host there is no Actions API at
all, and Forgejo 13 lists runs without serving their jobs or logs — so a chain
that assumed one shape would be wrong on most of the range.

The combined commit status is last and present everywhere. It is not only the
floor fallback: it is the **only** surface that sees CI running outside the
forge, which on self-hosted Forgejo is common — Woodpecker and Drone report
there and appear in `ci` like any other check.

Only a job with an id has a log, which is why the summary prints `job=<id>` for
some entries and not others — a commit-status check has nothing to fetch. Where
a release serves no job logs at all, the job's section reads `(no log
available)` instead of implying the id was wrong. Gitea answers an unknown job
id with HTTP 500 rather than 404 (measured on 1.24.7, against 404 on Forgejo
16.0.5); that is normalized on this one endpoint so an agent chases a stale id
instead of an imagined outage.

### Context cost

Every plugin tool is added to the agent's prompt and nothing can be removed to
make room, so the set is deliberately two tools and the descriptions are written
to be read once. Measured with Kandev's own estimator
(`o200k_base:mcp-tool-json-v1`, the same one behind `EstimatedTokens` in the MCP
attachment evidence):

| Tool | Tokens |
| --- | --- |
| `ci` | 150 |
| `pr` | 177 |
| **Total** | **327** |

For scale, Kandev's own `show_rich_output_kandev` is around 2k tokens by itself.

Log fetching is an argument on `ci` rather than a tool of its own. As a separate
`ci_log` the pair cost 419 tokens and answered "what failed and why" in two
round trips; folding it in costs 327 and answers in one. No `output_schema` is
declared either: it would be shipped to every session for results that are
already self-describing. `internal/plugin/manifest_test.go` holds a byte ceiling
on the set so a third tool has to be argued for.

The catalog is static, so this is the cost on every host. A plugin cannot
withhold a tool from an instance too old to serve it: Kandev builds the catalog
from the installed manifest and the only dynamic input is whether the plugin is
active. On Gitea 1.20 or Forgejo 7 the `logs` argument is therefore declared but
inert, which is one more reason it is an argument rather than a tool.

Whether this is a net win depends on how much prompt text it lets you delete.
Start an Autopilot task and read the recorded MCP attachment evidence, which
reports per-tool `EstimatedTokens`, rather than taking the table above on faith.

## Connection scope

**One connection serves every workspace.** The instance URL and token live in the
manifest's `config_schema`, which Kandev stores as a single plugin-level record
and renders at **Settings → Plugins → Forgejo**. Configure it once and it applies
everywhere.

The panel on the workspace integrations screen is a *status* surface, not a
second place to configure credentials: it reports whether the shared connection
is reachable and publishes that to the per-workspace enabled badge. It says so
on screen, because a per-workspace panel backed by a global credential is
otherwise easy to misread as a per-workspace setting.

If you need different Forgejo instances or different tokens per workspace, this
plugin does not support that today. It would mean moving the connection out of
`config_schema` and into workspace-scoped Host state and secrets, with explicit
save and disconnect actions — a deliberate change, not a configuration option.

## Enabling and disabling per workspace

The integrations card carries its own **enable** switch. Kandev does not render
one for a plugin integration — native integrations get theirs from the host, and
a plugin that wants parity has to supply it and publish the result with
`host.setIntegrationEnabled`. That is why this card had no toggle before 0.1.2.

The switch is not decorative. The choice is stored per workspace on the backend,
which honors it: while an integration is off, repository, branch, review and
association reads return nothing, so the native pickers render empty instead of
showing a provider the operator turned off, and pull-request create, link and
unlink refuse outright rather than silently doing nothing. Turning it back on
restores everything. A workspace with no stored choice is enabled — installing
the plugin is the opt-in.

Disabling is not uninstalling: config, credentials and stored task links all
survive, and the plugin stays installed for other workspaces.

## The "unsigned" badge

Every plugin shows as unsigned at **Settings → Plugins**, first-party ones
included. It is not a property of this package.

Kandev marks an install signed only when the package carries a
`checksums.txt.sig` **and** the host has a signature verifier wired
(`pkgtar.VerifySignature`). That hook is nil in the shipped product — it is
assigned only in the host's own tests — so the check short-circuits before any
signature is examined and every install is reported unsigned. The host is
deliberately honest here: it declines to claim a guarantee nothing verified.

This package therefore ships no `checksums.txt.sig`, and that is deliberate.
Adding one would not change the badge today, and it would add risk: a present
signature that fails verification fails the install outright, so a package
signed against one key would stop installing the day a host wires a verifier
expecting another. The archive's internal `checksums.txt` is still generated and
enforced on install, which detects corruption; it does not prove provenance.
Release provenance comes from the signed git tag and the GitHub release.

## Headless install and configuration

The UI flow above is the normal path. Everything it does is reachable over HTTP,
which is what scripted and headless installs need. All routes are Kandev's, not
this plugin's.

```sh
KANDEV=https://kandev.example.com
ID=kandev-plugin-forgejo
VERSION=0.1.2

# Install from a local package...
curl -sf -X POST "$KANDEV/api/plugins/install" -F "package=@$ID-$VERSION.tar.gz"
# ...or straight from a release URL.
curl -sf -X POST "$KANDEV/api/plugins/install" \
  -H 'Content-Type: application/json' \
  -d "{\"url\":\"https://github.com/naerymdan/$ID/releases/download/v$VERSION/$ID-$VERSION.tar.gz\"}"

# Configure. The `config` wrapper is required: a flat body is rejected with a
# misleading `missing required field "base_url"`.
curl -sf -X PATCH "$KANDEV/api/plugins/$ID" \
  -H 'Content-Type: application/json' \
  -d '{"config":{"base_url":"https://forgejo.example.com","api_token":"<token>"}}'

# Invoke an action. The `body` wrapper is required too: omitting it returns
# 503 `plugin action unavailable`, with the real cause (`unexpected end of
# JSON input`) only in the server log.
curl -sf -X POST "$KANDEV/api/plugins/$ID/actions/connection.test" \
  -H 'Content-Type: application/json' \
  -d '{"workspaceId":"<workspace-id>"}'
```

Every action in this plugin is `scope: "workspace"`, so **`workspaceId` is
required on all of them**. Kandev validates the envelope before dispatching, so
a missing selector fails with a 400 that never reaches the plugin process.

Action bodies:

| Action | Body |
| --- | --- |
| `connection.get` / `connection.test` | none |
| `connection.set_enabled` | `{"enabled":true}` |
| `repositories.list` | `{"query":"","cursor":"","limit":100}` |
| `repositories.inspect` | `{"url":"https://forgejo.example.com/owner/repo"}` |
| `repositories.branches` | `{"repository":{…full descriptor…}}` — a flat identity is rejected |
| `change_requests.get` / `.associations` | none |
| `change_requests.create` | `{"title","description","destination","draft"}` (also needs `taskId`, `sessionId`, `repositoryId`) |
| `change_requests.link` | `{"reference":"owner/repo#1"}` (also needs `taskId`) |
| `change_requests.unlink` | `{"connection_scope","repository_id","number"}` (also needs `taskId`) |

> **`DELETE /api/plugins/{id}` uninstalls immediately.** There is no
> confirmation step and it removes the plugin's config, state, and secrets. It
> is easy to hit while probing routes. This is Kandev's API, not this plugin's,
> but it is worth knowing before you script against it.

## Identity and security notes

- **Repository identity is the instance's immutable numeric id**, never
  `owner/name`. A rename or transfer does not break a stored link.
- **A task↔pull-request link is the full tuple** `(instance URL, repository id,
  pull request number)`, and each link is its own Host state entry — Host state
  has no compare-and-swap, so a shared per-task list would lose a concurrent
  link.
- `matchesURL` is only a hint; `inspectURL` performs an authenticated,
  workspace-scoped lookup and returns "not owned" rather than guessing.
- Pull-request creation takes its repository, session, and head branch from
  Kandev's `VerifiedActionContext`. The browser body may supply only title,
  description, destination, and draft state.
- Composer reference selection is not authorization: submission re-checks
  access against the instance and fails closed on any error or revocation.
- The access token never appears in responses, logs, cursors, repository
  descriptors, or error messages — provider error bodies are not forwarded,
  because some deployments echo the presented token.
- Declared capabilities are `api_read: [tasks, repositories]`, `state`, and
  `secrets`. There is no `api_write`: Kandev owns every entity mutation.

## Development

The Kandev SDK is not yet published as a standalone module, so this repo builds
against a **sibling checkout** of the Kandev monorepo:

```text
parent/
├── kandev/                  # github.com/kdlbs/kandev
└── kandev-plugin-forgejo/   # this repo
```

`go.mod` has `replace github.com/kandev/kandev => ../kandev/apps/backend`, and
`package.json` resolves `@kandev/plugin-sdk` from
`../kandev/apps/packages/plugin-sdk`. Both paths change once the SDK ships as a
versioned module.

```sh
npm install
make lint          # gofmt, go vet, tsc
make test          # Go unit tests + UI tests (builds the bundle first)
make verify-package-host   # host-platform tarball + checksum verification
make package       # all five platforms, for a release
```

### Running the live contract tests

The tests in `internal/forgejo/integration_test.go` skip unless an instance is
supplied. Use a **disposable** instance — never a production one:

```sh
podman run -d --name forgejo -p 3000:3000 \
  -e FORGEJO__security__INSTALL_LOCK=true \
  -e FORGEJO__database__DB_TYPE=sqlite3 \
  codeberg.org/forgejo/forgejo:16

# Create a user and a repo with a pull request, then mint the token through
# the admin CLI -- `--scopes all` is the one spelling that works on every
# supported version:
#   <binary> admin user generate-access-token --username kandev \
#     --token-name dev --scopes all --raw
KANDEV_FORGEJO_URL=http://localhost:3000 \
KANDEV_FORGEJO_TOKEN=... \
KANDEV_FORGEJO_OWNER=kandev KANDEV_FORGEJO_REPO=demo \
KANDEV_FORGEJO_HEAD_BRANCH=feature/kandev-create \
go test ./internal/forgejo -run TestLive -v
```

`KANDEV_FORGEJO_EXPECT_FLAVOR=forgejo|gitea` additionally asserts flavor
detection. The create and edit tests branch off a fresh head each run, so they
are safe to re-run against a long-lived instance. `TestLiveActionsSurfaceMatrix`
prints which Actions endpoints the instance serves, which is the table under
"What CI data you get, by version". Run the suite against Forgejo and Gitea
containers before releasing; CI does this across the supported range.

## Layout

| Path | Contents |
| --- | --- |
| `manifest.yaml` | The declarative host contract: actions, provider ownership, reference source, capabilities, config schema. |
| `internal/sourcecontrol/` | The provider-neutral source-control recipe — the Kandev boundary. Adapted from `kandev-plugin-template`. |
| `internal/forgejo/` | Concrete Forgejo/Gitea adapters: REST client, repositories, pull requests, reviews, references, associations, and the Actions/CI resolution chain. |
| `internal/plugin/` | Wires the adapters into the extension, owns the connection actions, and serves the agent tools. |
| `ui/src/` | The browser half: the recipe registration plus the Forgejo icon, reference parsing, detail adapter, and connection panel. |
| `server/` | The `pluginsdk.Serve` entry point Kandev spawns. |

## License

MIT — see [LICENSE](LICENSE).

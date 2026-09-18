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
| Review panel + CI popover | Review state, approval counts, individual commit statuses, and unresolved review comments, on desktop and mobile. |
| Composer `#` references | Search pull requests from the composer; access is re-checked live at submit time. |

## Requirements

- Kandev **0.88.0** or newer. The provider-neutral source-control contracts this
  plugin builds on landed after `v0.87.1` and first shipped in `v0.88.0`.
- A Forgejo or Gitea instance reachable from the Kandev backend, serving the
  REST v1 API at `<instance URL>/api/v1`.
- A personal access token with `read:repository` (add `write:repository` to open
  pull requests).

## Install

1. Download `kandev-plugin-forgejo-<version>.tar.gz` from
   [Releases](https://github.com/naerymdan/kandev-plugin-forgejo/releases).
2. In Kandev: **Settings → Plugins → Install from file**.
3. Open **Settings → Plugins → Forgejo** and set:
   - **Instance URL** — e.g. `https://codeberg.org` or `http://forge.lan:3000`.
   - **Access token** — stored in Kandev's encrypted vault and readable only
     inside the plugin process.
4. Use **Test connection** in **Settings → Integrations → Forgejo** to confirm
   the instance is reachable.

Changing the configuration restarts the plugin; that is expected.

## Forgejo and Gitea

Forgejo is a hard fork of Gitea and both serve the same versioned REST surface
at `/api/v1`. This plugin deliberately restricts itself to endpoints and fields
that exist on both, so one plugin serves either host. The connection panel
labels which flavor it detected, but no behavior branches on it.

Two places where the shared surface differs from what a GitHub-shaped client
would assume, and which this plugin handles explicitly:

- A combined commit status uses `state`, but each entry inside it uses
  `status`. Reading `state` on the entries yields no checks at all.
- REST v1 has **no draft flag** when creating a pull request. Draft is expressed
  with a `WIP:` title prefix, which both web UIs recognize; an existing marker
  is not doubled.

### Verified against

The live contract tests in `internal/forgejo/integration_test.go` were run
against both, with identical results:

| Host | Version |
| --- | --- |
| Forgejo | 13.0.5+gitea-1.22.0 |
| Gitea | 1.24.7 |

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
  codeberg.org/forgejo/forgejo:13

# create a user, a token, a repo with a pull request, then:
KANDEV_FORGEJO_URL=http://localhost:3000 \
KANDEV_FORGEJO_TOKEN=... \
KANDEV_FORGEJO_OWNER=kandev KANDEV_FORGEJO_REPO=demo \
KANDEV_FORGEJO_HEAD_BRANCH=feature/kandev-create \
go test ./internal/forgejo -run TestLive -v
```

Run the same command against a Gitea container before releasing.

## Layout

| Path | Contents |
| --- | --- |
| `manifest.yaml` | The declarative host contract: actions, provider ownership, reference source, capabilities, config schema. |
| `internal/sourcecontrol/` | The provider-neutral source-control recipe — the Kandev boundary. Adapted from `kandev-plugin-template`. |
| `internal/forgejo/` | Concrete Forgejo/Gitea adapters: REST client, repositories, pull requests, reviews, references, associations. |
| `internal/plugin/` | Wires the adapters into the extension and owns the connection actions. |
| `ui/src/` | The browser half: the recipe registration plus the Forgejo icon, reference parsing, detail adapter, and connection panel. |
| `server/` | The `pluginsdk.Serve` entry point Kandev spawns. |

## License

MIT — see [LICENSE](LICENSE).

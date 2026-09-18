# Changelog

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
- Flavor detection probes Forgejo's `/api/forgejo/v1` namespace rather than
  matching the `+gitea-` version suffix, so it survives further divergence.
- Verified against Forgejo 13.0.5, Forgejo 16.0.5, and Gitea 1.24.7, with
  identical results on all three.

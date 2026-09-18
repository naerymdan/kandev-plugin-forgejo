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

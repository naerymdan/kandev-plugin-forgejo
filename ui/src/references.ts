/**
 * Client-side shape check for a pasted pull-request reference.
 *
 * This is a fast-fail for the link dialog only. The backend re-resolves every
 * reference against the instance before anything is stored, so this function is
 * deliberately permissive about hosts: it cannot know which instance the
 * operator configured.
 */
// Pull request numbers are 1-based; `#0` must fail here rather than
// round-tripping to the backend only to be rejected there.
const OWNER_REPO_NUMBER = /^[^/\s]+\/[^#\s]+#[1-9]\d*$/;
const PULL_REQUEST_PATH = /\/([^/\s]+)\/([^/\s]+)\/pulls\/([1-9]\d*)(?:[/?#].*)?$/;

export function parsePullRequestReference(reference: string): string | null {
  const trimmed = reference.trim();
  if (!trimmed) return null;

  if (OWNER_REPO_NUMBER.test(trimmed)) return trimmed;

  if (!trimmed.includes("://")) return null;
  let url: URL;
  try {
    url = new URL(trimmed);
  } catch {
    return null;
  }
  const match = PULL_REQUEST_PATH.exec(url.pathname);
  if (!match) return null;
  const [, owner, repository, number] = match;
  if (!owner || !repository || !number || Number(number) <= 0) return null;
  // Forward the absolute URL so the backend can verify it against the
  // configured instance host rather than trusting this parse.
  return trimmed;
}

/** True when a URL could plausibly belong to the configured instance. */
export function looksLikeRepositoryURL(url: string): boolean {
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

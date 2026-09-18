import { describe, expect, it } from "vitest";
import { looksLikeRepositoryURL, parsePullRequestReference } from "../src/references";

describe("parsePullRequestReference", () => {
  it("accepts the canonical owner/repo#number form", () => {
    expect(parsePullRequestReference("acme/widgets#42")).toBe("acme/widgets#42");
    expect(parsePullRequestReference("  acme/widgets#42  ")).toBe("acme/widgets#42");
  });

  it("accepts a pull request URL and forwards it for server-side verification", () => {
    const url = "https://forge.example.com/acme/widgets/pulls/42";
    expect(parsePullRequestReference(url)).toBe(url);
    expect(parsePullRequestReference(`${url}/files`)).toBe(`${url}/files`);
  });

  it("rejects references that cannot identify a pull request", () => {
    // A bare number has no repository, so the identity would be ambiguous.
    expect(parsePullRequestReference("#42")).toBeNull();
    expect(parsePullRequestReference("42")).toBeNull();
    expect(parsePullRequestReference("acme/widgets#0")).toBeNull();
    expect(parsePullRequestReference("https://forge.example.com/acme/widgets/issues/42")).toBeNull();
    expect(parsePullRequestReference("https://forge.example.com/acme/widgets")).toBeNull();
    expect(parsePullRequestReference("not a url")).toBeNull();
    expect(parsePullRequestReference("   ")).toBeNull();
  });

  it("does not guess the instance host", () => {
    // This parser cannot know which instance is configured; the backend
    // re-resolves against it, so a foreign-looking host still passes here.
    expect(parsePullRequestReference("https://codeberg.org/acme/widgets/pulls/7")).toBe(
      "https://codeberg.org/acme/widgets/pulls/7",
    );
  });
});

describe("looksLikeRepositoryURL", () => {
  it("accepts http(s) and scp-style remotes", () => {
    expect(looksLikeRepositoryURL("https://forge.example.com/acme/widgets")).toBe(true);
    expect(looksLikeRepositoryURL("http://forge.lan:3000/acme/widgets.git")).toBe(true);
    expect(looksLikeRepositoryURL("git@forge.example.com:acme/widgets.git")).toBe(true);
  });

  it("rejects values that are not remotes", () => {
    expect(looksLikeRepositoryURL("")).toBe(false);
    expect(looksLikeRepositoryURL("   ")).toBe(false);
    expect(looksLikeRepositoryURL("acme/widgets")).toBe(false);
    expect(looksLikeRepositoryURL("ftp://forge.example.com/a/b")).toBe(false);
  });
});

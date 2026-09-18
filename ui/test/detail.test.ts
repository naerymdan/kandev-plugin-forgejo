import { describe, expect, it } from "vitest";
import type { ReviewSummary } from "@kandev/plugin-sdk";
import { toChangeRequestDetail } from "../src/detail";

function review(overrides: Partial<ReviewSummary> = {}): ReviewSummary {
  return {
    providerId: "forgejo",
    reviewKey: "https://forge.example.com/repositories/7/pulls/42",
    title: "Add widgets",
    url: "https://forge.example.com/acme/widgets/pulls/42",
    connectionScope: "https://forge.example.com",
    repositoryId: "7",
    changeRequestNumber: 42,
    state: "open",
    ...overrides,
  } as ReviewSummary;
}

describe("toChangeRequestDetail", () => {
  it("maps a full snapshot onto the host detail props", () => {
    const detail = toChangeRequestDetail(
      review({
        taskStatus: {
          number: 42,
          state: "open",
          pipelineState: "success",
          checks: [{ id: "1", label: "ci/build", state: "success", detail: "ok", url: "https://ci/1" }],
          review: { state: "approved", approved: 2 },
          unresolvedComments: 3,
          updatedAt: 1755002400000,
        },
      }),
    ) as Record<string, unknown>;

    expect(detail.provider).toBe("forgejo");
    expect(detail.number).toBe(42);
    expect(detail.title).toBe("Add widgets");
    expect(detail.state).toBe("open");
    expect(detail.pipelineState).toBe("success");
    expect(detail.checks).toHaveLength(1);
    expect(detail.review).toEqual({ state: "approved", approved: 2 });
    expect(detail.unresolvedComments).toBe(3);
    expect(detail.updatedAt).toBe(1755002400000);
  });

  it("falls back to summary fields when no task status is present", () => {
    const detail = toChangeRequestDetail(review()) as Record<string, unknown>;
    expect(detail.number).toBe(42);
    expect(detail.state).toBe("open");
    expect(detail.pipelineState).toBe("neutral");
    expect(detail.checks).toEqual([]);
    // Absent optional fields must be omitted, not set to undefined.
    expect("review" in detail).toBe(false);
    expect("unresolvedComments" in detail).toBe(false);
    expect("updatedAt" in detail).toBe(false);
  });

  it("omits optional check fields that are absent", () => {
    const detail = toChangeRequestDetail(
      review({
        taskStatus: {
          number: 42,
          state: "merged",
          pipelineState: "neutral",
          checks: [{ id: "1", label: "ci/build", state: "pending" }],
        },
      }),
    ) as { checks: Record<string, unknown>[] };
    expect(detail.checks[0]).toEqual({ id: "1", label: "ci/build", state: "pending" });
  });
});

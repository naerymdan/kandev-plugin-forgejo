import type { ReviewSummary } from "@kandev/plugin-sdk";

/**
 * The one provider-specific presentation adapter the recipe asks for: it turns
 * a normalized review snapshot into the props `host.ui.ChangeRequestDetail`
 * renders. The host owns the layout for both desktop and mobile, so this stays
 * small and is covered by the packaged-plugin smoke test.
 */
export function toChangeRequestDetail(review: ReviewSummary): unknown {
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
      ...(check.detail ? { detail: check.detail } : {}),
      ...(check.url ? { url: check.url } : {}),
    })),
    ...(status?.review
      ? {
          review: {
            state: status.review.state,
            approved: status.review.approved,
            ...(status.review.required === undefined ? {} : { required: status.review.required }),
            ...(status.review.requested === undefined ? {} : { requested: status.review.requested }),
          },
        }
      : {}),
    ...(status?.unresolvedComments === undefined
      ? {}
      : { unresolvedComments: status.unresolvedComments }),
    ...(status?.updatedAt === undefined ? {} : { updatedAt: status.updatedAt }),
  };
}

import { keys } from "$lib/api/keys";
import { readDecisionRight } from "$lib/executions/decision.server";
import { readDiff } from "$lib/executions/diff.server";
import type { ReviewView } from "$lib/executions/review";
import { reviewPreviewStates } from "./preview";
import type { PageServerLoad } from "./$types";

export type ReviewPageData = { review: ReviewView };

export const load: PageServerLoad = async ({
	depends,
	route,
	locals,
	params,
	parent,
	url,
}): Promise<ReviewPageData> => {
	depends(keys.page(route.id));

	const { workspace } = await parent();

	if (import.meta.env.DEV && reviewPreviewStates[url.searchParams.get("state") ?? ""]) {
		return { review: { kind: "loading" } };
	}

	depends(keys.execution(params.executionId), keys.executionReview(params.executionId));

	const path = { workspaceId: workspace.id, executionId: params.executionId };
	const requested = Number(url.searchParams.get("revision"));
	const revision = Number.isInteger(requested) && requested >= 1 ? requested : undefined;

	const [detail, review, questions] = await Promise.all([
		locals.api.GET("/workspaces/{workspaceId}/executions/{executionId}", { params: { path } }),
		locals.api.GET("/workspaces/{workspaceId}/executions/{executionId}/review", {
			params: { path, query: { revision } },
		}),
		locals.api.GET("/workspaces/{workspaceId}/executions/{executionId}/questions", {
			params: { path },
		}),
	]);

	if (detail.error?.status === 404) return { review: { kind: "not_found" } };
	if (revision && review.error?.status === 404) {
		return { review: { kind: "revision_not_found", executionId: params.executionId } };
	}
	if (!detail.data || !review.data) return { review: { kind: "unavailable" } };

	const repositories = review.data.repositories;

	const [right, ...diffs] = await Promise.all([
		readDecisionRight(locals.api, workspace.id, detail.data.execution.issueId),
		...repositories.map((held) =>
			readDiff(locals.api, workspace.id, params.executionId, held.diffArtifactId)
		),
	]);

	return {
		review: {
			kind: "ready",
			execution: detail.data.execution,
			review: review.data,
			questions: questions.data?.questions ?? [],
			right,
			repositories: repositories.map((held, index) => ({
				repository: held.repository,
				branch: held.branch || undefined,
				baseSha: held.baseSha,
				headSha: held.headSha,
				commits: held.commits,
				additions: held.additions,
				deletions: held.deletions,
				artifactId: held.diffArtifactId,
				diff: diffs[index],
			})),
		},
	};
};

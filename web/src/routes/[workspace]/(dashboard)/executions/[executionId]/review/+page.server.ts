import { keys } from "$lib/api/keys";
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

	const [detail, review, questions] = await Promise.all([
		locals.api.GET("/workspaces/{workspaceId}/executions/{executionId}", { params: { path } }),
		locals.api.GET("/workspaces/{workspaceId}/executions/{executionId}/review", {
			params: { path },
		}),
		locals.api.GET("/workspaces/{workspaceId}/executions/{executionId}/questions", {
			params: { path },
		}),
	]);

	if (detail.error?.status === 404) return { review: { kind: "not_found" } };
	if (!detail.data || !review.data) return { review: { kind: "unavailable" } };

	const changes = detail.data.changeset?.repositories ?? [];

	const diffs = await Promise.all(
		changes.map((change) =>
			readDiff(locals.api, workspace.id, params.executionId, change.diffArtifactId)
		)
	);

	return {
		review: {
			kind: "ready",
			execution: detail.data.execution,
			changeset: detail.data.changeset,
			review: review.data,
			questions: questions.data?.questions ?? [],
			repositories: changes.map((change, index) => ({
				repository: change.repository,
				branch: change.branch,
				headSha: change.headSha ?? "",
				additions: change.additions,
				deletions: change.deletions,
				artifactId: change.diffArtifactId,
				diff: diffs[index],
			})),
		},
	};
};

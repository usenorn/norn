import { keys } from "$lib/api/keys";
import { readDecisionRight } from "$lib/executions/decision.server";
import type { RunView } from "$lib/executions/executions";
import { runPreviewStates } from "./preview";
import type { PageServerLoad } from "./$types";

export type RunPageData = { run: RunView };

export const load: PageServerLoad = async ({
	depends,
	route,
	locals,
	params,
	parent,
	url,
}): Promise<RunPageData> => {
	depends(keys.page(route.id));

	const { workspace } = await parent();

	if (import.meta.env.DEV && runPreviewStates[url.searchParams.get("state") ?? ""]) {
		return { run: { kind: "loading" } };
	}

	depends(keys.execution(params.executionId));

	const path = { workspaceId: workspace.id, executionId: params.executionId };

	const detail = await locals.api.GET("/workspaces/{workspaceId}/executions/{executionId}", {
		params: { path },
	});

	if (detail.error?.status === 404) return { run: { kind: "not_found" } };
	if (!detail.data) return { run: { kind: "unavailable" } };

	const [questions, previews, links, plans, right] = await Promise.all([
		locals.api.GET("/workspaces/{workspaceId}/executions/{executionId}/questions", {
			params: { path },
		}),
		locals.api.GET("/workspaces/{workspaceId}/executions/{executionId}/previews", {
			params: { path },
		}),
		locals.api.GET("/workspaces/{workspaceId}/issues/{issueId}/code-links", {
			params: { path: { workspaceId: workspace.id, issueId: detail.data.execution.issueId } },
		}),
		locals.api.GET("/workspaces/{workspaceId}/executions/{executionId}/plans", {
			params: { path },
		}),
		readDecisionRight(locals.api, workspace.id, detail.data.execution.issueId),
	]);

	return {
		run: {
			kind: "ready",
			execution: detail.data.execution,
			timeline: detail.data.timeline,
			services: detail.data.services ?? [],
			previews:
				previews.data ??
				(detail.data.previews ?? []).map((preview) => ({ preview, shareLinks: [] })),
			runner: detail.data.runner,
			changeset: detail.data.changeset,
			codeLinks: links.data ?? [],
			plans: plans.data ?? [],
			plansReach: plans.data ? "loaded" : "unavailable",
			right,
			questions: questions.data?.questions ?? [],
			questionsReach: questions.data ? "loaded" : "unavailable",
		},
	};
};

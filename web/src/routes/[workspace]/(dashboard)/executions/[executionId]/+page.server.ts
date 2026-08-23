import { keys } from "$lib/api/keys";
import { chunkPageSize, type RunView } from "$lib/executions/executions";
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

	const [questions, transcript, logs] = await Promise.all([
		locals.api.GET("/workspaces/{workspaceId}/executions/{executionId}/questions", {
			params: { path },
		}),
		locals.api.GET("/workspaces/{workspaceId}/executions/{executionId}/transcript", {
			params: { path, query: { limit: chunkPageSize } },
		}),
		locals.api.GET("/workspaces/{workspaceId}/executions/{executionId}/logs", {
			params: { path, query: { limit: chunkPageSize } },
		}),
	]);

	const transcriptChunks = transcript.data ?? [];
	const logChunks = logs.data ?? [];

	return {
		run: {
			kind: "ready",
			execution: detail.data.execution,
			timeline: detail.data.timeline,
			services: detail.data.services ?? [],
			previews: detail.data.previews ?? [],
			runner: detail.data.runner,
			questions: questions.data?.questions ?? [],
			transcript: transcriptChunks.flatMap((chunk) => chunk.entries),
			logs: logChunks.flatMap((chunk) => chunk.entries),
			transcriptCursor: transcriptChunks.at(-1)?.sequence,
			logCursor: logChunks.at(-1)?.sequence,
		},
	};
};

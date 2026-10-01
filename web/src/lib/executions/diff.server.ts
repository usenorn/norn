import type { Client } from "openapi-fetch";
import type { paths } from "$lib/api/dashboard.gen";
import { internalOrigin } from "$lib/api/server";
import { reviewBudget } from "./diff";
import { highlighted } from "./highlight.server";
import { readPatch } from "./patch.server";
import type { RepositoryDiff } from "./review";

export async function storedPatch(
	api: Client<paths>,
	workspaceId: string,
	executionId: string,
	artifactId: string
): Promise<Response | null> {
	const answered = await api.GET(
		"/workspaces/{workspaceId}/executions/{executionId}/artifacts/{artifactId}/content",
		{ params: { path: { workspaceId, executionId, artifactId } }, redirect: "manual" }
	);

	const target = answered.response.headers.get("location");

	if (!target) return null;

	const stored = await fetch(new URL(target, internalOrigin()));

	return stored.ok ? stored : null;
}

export async function readDiff(
	api: Client<paths>,
	workspaceId: string,
	executionId: string,
	artifactId: string | undefined
): Promise<RepositoryDiff> {
	if (!artifactId) return { kind: "absent" };

	try {
		const stored = await storedPatch(api, workspaceId, executionId, artifactId);

		if (!stored) return { kind: "failed" };

		const read = await readPatch(stored, reviewBudget);

		return { kind: "ready", files: await highlighted(read.files), truncated: read.truncated };
	} catch {
		return { kind: "failed" };
	}
}

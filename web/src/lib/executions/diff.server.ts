import type { Client } from "openapi-fetch";
import type { paths } from "$lib/api/dashboard.gen";
import { internalOrigin } from "$lib/api/server";
import { parseDiff } from "./diff";
import type { RepositoryDiff } from "./review";

export const diffMaxBytes = 512 * 1024;

async function unpack(stored: Response): Promise<string> {
	const bytes = new Uint8Array(await stored.arrayBuffer());

	if (bytes[0] !== 0x1f || bytes[1] !== 0x8b) return new TextDecoder().decode(bytes);

	const unpacked = new Blob([bytes.slice()]).stream().pipeThrough(new DecompressionStream("gzip"));

	return new Response(unpacked).text();
}

function cut(patch: string): string {
	const lastWholeFile = patch.lastIndexOf("\ndiff --git ", diffMaxBytes);

	return patch.slice(0, lastWholeFile > 0 ? lastWholeFile : diffMaxBytes);
}

export async function readDiff(
	api: Client<paths>,
	workspaceId: string,
	executionId: string,
	artifactId: string | undefined
): Promise<RepositoryDiff> {
	if (!artifactId) return { kind: "absent" };

	const answered = await api.GET(
		"/workspaces/{workspaceId}/executions/{executionId}/artifacts/{artifactId}/content",
		{ params: { path: { workspaceId, executionId, artifactId } }, redirect: "manual" }
	);

	const target = answered.response.headers.get("location");

	if (!target) return { kind: "absent" };

	try {
		const stored = await fetch(new URL(target, internalOrigin()));

		if (!stored.ok) return { kind: "failed" };

		const patch = await unpack(stored);
		const truncated = patch.length > diffMaxBytes;

		return { kind: "ready", files: parseDiff(truncated ? cut(patch) : patch), truncated };
	} catch {
		return { kind: "failed" };
	}
}

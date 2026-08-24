import { error } from "@sveltejs/kit";
import { reachOfSlug } from "$lib/account/accounts";
import { internalOrigin } from "$lib/api/server";
import { diffMaxBytes } from "$lib/executions/executions";
import type { RequestHandler } from "./$types";

async function unpack(stored: Response): Promise<string> {
	const bytes = new Uint8Array(await stored.arrayBuffer());

	if (bytes[0] !== 0x1f || bytes[1] !== 0x8b) return new TextDecoder().decode(bytes);

	const unpacked = new Blob([bytes.slice()]).stream().pipeThrough(new DecompressionStream("gzip"));

	return new Response(unpacked).text();
}

export const GET: RequestHandler = async ({ locals, params }) => {
	const reach = reachOfSlug(await locals.signedIn, params.workspace, await locals.acting);

	if (!reach) error(404, "That workspace does not exist, or you are not a member of it.");

	const answered = await locals.api.GET(
		"/workspaces/{workspaceId}/executions/{executionId}/artifacts/{artifactId}/content",
		{
			params: {
				path: {
					workspaceId: reach.workspace.workspace.id,
					executionId: params.executionId,
					artifactId: params.artifactId,
				},
			},
			redirect: "manual",
		}
	);

	const target = answered.response.headers.get("location");

	if (!target) error(404, "That diff is no longer kept.");

	const stored = await fetch(new URL(target, internalOrigin()));

	if (!stored.ok) error(502, "The diff could not be read.");

	const patch = await unpack(stored);
	const truncated = patch.length > diffMaxBytes;

	return new Response(truncated ? patch.slice(0, diffMaxBytes) : patch, {
		headers: {
			"content-type": "text/plain; charset=utf-8",
			"cache-control": "no-store",
			"x-diff-truncated": truncated ? "true" : "false",
		},
	});
};

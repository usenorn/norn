import { getRequestEvent, query } from "$app/server";
import { z } from "zod";
import { diffFileMax, type FileDiff } from "./diff";
import { storedPatch } from "./diff.server";
import { highlighted } from "./highlight.server";
import { readPatch } from "./patch.server";

const fileAsked = z.object({
	workspaceId: z.uuid(),
	executionId: z.string().min(1).max(100),
	artifactId: z.uuid(),
	path: z.string().min(1).max(4096),
});

export const fileDiff = query(fileAsked, async ({ workspaceId, executionId, artifactId, path }): Promise<FileDiff> => {
	const { locals } = getRequestEvent();

	try {
		const stored = await storedPatch(locals.api, workspaceId, executionId, artifactId);

		if (!stored) return { kind: "failed" };

		const read = await readPatch(stored, { inline: Infinity, perFile: diffFileMax, files: Infinity, only: path });
		const [file] = read.files;

		if (!file) return { kind: "failed" };

		if (read.oversized) return { kind: "too_large", lines: file.additions + file.deletions };

		const [shown] = await highlighted([file]);

		return { kind: "ready", file: shown };
	} catch {
		return { kind: "failed" };
	}
});

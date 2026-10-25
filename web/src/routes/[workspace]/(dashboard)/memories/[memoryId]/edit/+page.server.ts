import { fail, redirect } from "@sveltejs/kit";
import { message, superValidate, type Infer } from "sveltekit-superforms";
import { zod4 } from "sveltekit-superforms/adapters";
import { keys } from "#lib/api/keys.js";
import {
	memoryContentFields,
	memoryContentOf,
	memoryPath,
	readMemoryFailure,
	type MemoryDraft,
	type MemoryFailure,
} from "#lib/memories/memories.js";
import { readMemoryDraft, readProjects } from "#lib/memories/memories.server.js";
import { correctMemoryFormId, correctMemorySchema } from "#lib/memories/memory-schema.js";
import { correctMemoryPreviewStates } from "./preview";
import type { Actions, PageServerLoad } from "./$types";

type CorrectForm = Infer<typeof correctMemorySchema>;

export const load: PageServerLoad = async ({ depends, locals, params, parent, url }) => {
	const { workspace } = await parent();

	depends(keys.memory(params.memoryId));

	if (import.meta.env.DEV && correctMemoryPreviewStates[url.searchParams.get("state") ?? ""]?.draft) {
		const form = await superValidate<CorrectForm, MemoryFailure>(zod4(correctMemorySchema), { id: correctMemoryFormId });

		return { draft: { kind: "loading" } as MemoryDraft, form, projects: [] };
	}

	const [draft, projects] = await Promise.all([
		readMemoryDraft(locals.api, workspace.id, params.memoryId, false),
		readProjects(locals.api, workspace.id),
	]);

	const form =
		draft.kind === "ready"
			? await superValidate<CorrectForm, MemoryFailure>(
					{ ...memoryContentFields(draft.memory), expectedVersion: draft.memory.version },
					zod4(correctMemorySchema),
					{ id: correctMemoryFormId, errors: false }
				)
			: await superValidate<CorrectForm, MemoryFailure>(zod4(correctMemorySchema), { id: correctMemoryFormId });

	return { draft, form, projects };
};

export const actions: Actions = {
	default: async ({ locals, params, request }) => {
		const body = await request.formData();
		const form = await superValidate<CorrectForm, MemoryFailure>(body, zod4(correctMemorySchema), {
			id: correctMemoryFormId,
		});

		if (!form.valid) return fail(400, { form });

		const workspaceId = String(body.get("workspaceId") ?? "");

		if (!workspaceId) return message(form, { kind: "unavailable" }, { status: 400 });

		const path = { workspaceId, memoryId: params.memoryId };
		const current = await locals.api.GET("/workspaces/{workspaceId}/memories/{memoryId}", { params: { path } });

		if (!current.data) return message(form, readMemoryFailure(current.error), { status: 400 });

		const { error } = await locals.api.PUT("/workspaces/{workspaceId}/memories/{memoryId}", {
			params: { path },
			body: { ...memoryContentOf(form.data, current.data.evidence), expectedVersion: form.data.expectedVersion },
		});

		if (error) return message(form, readMemoryFailure(error), { status: 400 });

		redirect(303, memoryPath(params.workspace, params.memoryId));
	},
};

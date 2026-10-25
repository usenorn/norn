import { fail, redirect } from "@sveltejs/kit";
import { message, superValidate, type Infer } from "sveltekit-superforms";
import { zod4 } from "sveltekit-superforms/adapters";
import { keys } from "#lib/api/keys.js";
import type { DocAudience } from "#lib/docs/docs.js";
import {
	memoryContentFields,
	memoryContentOf,
	memoryPath,
	readMemoryFailure,
	type MemoryDraft,
	type MemoryFailure,
} from "#lib/memories/memories.js";
import { readMemoryDraft, readProjects } from "#lib/memories/memories.server.js";
import { supersedeMemoryFormId, supersedeMemorySchema } from "#lib/memories/memory-schema.js";
import { supersedeMemoryPreviewStates } from "./preview";
import type { Actions, PageServerLoad } from "./$types";

type SupersedeForm = Infer<typeof supersedeMemorySchema>;

export const load: PageServerLoad = async ({ depends, locals, params, parent, url }) => {
	const { workspace } = await parent();

	depends(keys.memory(params.memoryId));

	if (import.meta.env.DEV && supersedeMemoryPreviewStates[url.searchParams.get("state") ?? ""]?.draft) {
		const form = await superValidate<SupersedeForm, MemoryFailure>(zod4(supersedeMemorySchema), {
			id: supersedeMemoryFormId,
		});

		return { draft: { kind: "loading" } as MemoryDraft, form, projects: [] };
	}

	const [draft, projects] = await Promise.all([
		readMemoryDraft(locals.api, workspace.id, params.memoryId, true),
		readProjects(locals.api, workspace.id),
	]);

	const form =
		draft.kind === "ready"
			? await superValidate<SupersedeForm, MemoryFailure>(
					{
						...memoryContentFields(draft.memory),
						effectiveFrom: "",
						effectiveUntil: "",
						visibility: draft.memory.audience.visibility,
						projectId: draft.memory.audience.projectId ?? "",
						expectedVersion: draft.memory.version,
					},
					zod4(supersedeMemorySchema),
					{ id: supersedeMemoryFormId, errors: false }
				)
			: await superValidate<SupersedeForm, MemoryFailure>(zod4(supersedeMemorySchema), { id: supersedeMemoryFormId });

	return { draft, form, projects };
};

export const actions: Actions = {
	default: async ({ locals, params, request }) => {
		const body = await request.formData();
		const form = await superValidate<SupersedeForm, MemoryFailure>(body, zod4(supersedeMemorySchema), {
			id: supersedeMemoryFormId,
		});

		if (!form.valid) return fail(400, { form });

		const workspaceId = String(body.get("workspaceId") ?? "");

		if (!workspaceId) return message(form, { kind: "unavailable" }, { status: 400 });

		const audience: DocAudience =
			form.data.visibility === "project"
				? { visibility: "project", projectId: form.data.projectId }
				: { visibility: form.data.visibility };

		const { data, error } = await locals.api.POST("/workspaces/{workspaceId}/memories/{memoryId}/supersede", {
			params: { path: { workspaceId, memoryId: params.memoryId } },
			body: {
				expectedVersion: form.data.expectedVersion,
				replacement: { ...memoryContentOf(form.data), audience },
			},
		});

		if (error || !data) return message(form, readMemoryFailure(error), { status: 400 });

		redirect(303, memoryPath(params.workspace, data.id));
	},
};

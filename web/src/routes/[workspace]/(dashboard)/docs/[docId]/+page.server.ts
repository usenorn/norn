import { fail, redirect } from "@sveltejs/kit";
import { message, superValidate, type Infer } from "sveltekit-superforms";
import { zod4 } from "sveltekit-superforms/adapters";
import { keys } from "$lib/api/keys";
import { archiveDocSchema, archiveFormId } from "$lib/docs/doc-schema";
import { docPath, docsPath, readDocFailure, type DocDetail, type DocFailure } from "$lib/docs/docs";
import { readAgents, readDoc, readReaders } from "$lib/docs/docs.server";
import { docPreviewStates } from "./preview";
import type { Actions, PageServerLoad } from "./$types";

type ArchiveForm = Infer<typeof archiveDocSchema>;

export const load: PageServerLoad = async ({ depends, route, locals, params, parent, url }) => {
	depends(keys.page(route.id));

	const { workspace } = await parent();

	depends(keys.doc(params.docId));

	const forms = {
		archiveForm: await superValidate<ArchiveForm, DocFailure>(zod4(archiveDocSchema), { id: archiveFormId }),
	};

	if (import.meta.env.DEV && docPreviewStates[url.searchParams.get("state") ?? ""]) {
		return { detail: { kind: "loading" } as DocDetail, ...forms };
	}

	const read = await readDoc(locals.api, workspace.id, params.docId);

	if (read.kind !== "ready") return { detail: read as DocDetail, ...forms };

	const { doc } = read;

	const [revisions, readers] = await Promise.all([
		locals.api.GET("/workspaces/{workspaceId}/docs/{docId}/revisions", {
			params: { path: { workspaceId: workspace.id, docId: doc.id }, query: { limit: 1 } },
		}),
		readReaders(locals.api, workspace.id, doc.audience, {
			accountId: doc.authorAccountId,
			name: doc.authorName,
		}),
	]);

	const latest = revisions.data?.revisions[0];
	const agents = latest?.source === "agent" ? await readAgents(locals.api, workspace.id) : [];

	const detail: DocDetail = { kind: "ready", doc, latest, agents, readers };

	return { detail, ...forms };
};

export const actions: Actions = {
	archive: async ({ locals, params, request }) => {
		const body = await request.formData();
		const form = await superValidate<ArchiveForm, DocFailure>(body, zod4(archiveDocSchema), {
			id: archiveFormId,
		});

		if (!form.valid) return fail(400, { archiveForm: form });

		const workspaceId = String(body.get("workspaceId") ?? "");

		if (!workspaceId) return message(form, { kind: "unavailable" }, { status: 400 });

		const { data, error } = await locals.api.DELETE("/workspaces/{workspaceId}/docs/{docId}", {
			params: {
				path: { workspaceId, docId: params.docId },
				query: { expectedVersion: form.data.expectedVersion },
			},
		});

		if (error || !data) {
			return message(form, readDocFailure(error), { status: 400 });
		}

		redirect(303, data.parentId ? docPath(params.workspace, data.parentId) : docsPath(params.workspace));
	},
};

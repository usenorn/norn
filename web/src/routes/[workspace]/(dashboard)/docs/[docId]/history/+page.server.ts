import { fail, redirect } from "@sveltejs/kit";
import { message, superValidate, type Infer } from "sveltekit-superforms";
import { zod4 } from "sveltekit-superforms/adapters";
import { keys } from "#lib/api/keys.js";
import { restoreDocSchema, restoreFormId } from "#lib/docs/doc-schema.js";
import {
	docPath,
	docRevisionPageSize,
	readDocFailure,
	type DocChosen,
	type DocFailure,
	type DocHistory,
	type DocRevisionSummary,
} from "#lib/docs/docs.js";
import { readAgents, readDoc } from "#lib/docs/docs.server.js";
import { docHistoryPreviewStates } from "./preview";
import type { Actions, PageServerLoad } from "./$types";

type RestoreForm = Infer<typeof restoreDocSchema>;

export const load: PageServerLoad = async ({ depends, route, locals, params, parent, url }) => {
	depends(keys.page(route.id));

	const { workspace } = await parent();

	depends(keys.doc(params.docId));

	const emptyForm = await superValidate<RestoreForm, DocFailure>(zod4(restoreDocSchema), { id: restoreFormId });

	if (import.meta.env.DEV && docHistoryPreviewStates[url.searchParams.get("state") ?? ""]) {
		return { history: { kind: "loading" } as DocHistory, restoreForm: emptyForm };
	}

	const read = await readDoc(locals.api, workspace.id, params.docId);

	if (read.kind !== "ready") return { history: read as DocHistory, restoreForm: emptyForm };

	const path = { workspaceId: workspace.id, docId: read.doc.id };
	const cursor = url.searchParams.get("cursor") ?? undefined;

	const listed = await locals.api.GET("/workspaces/{workspaceId}/docs/{docId}/revisions", {
		params: { path, query: { limit: docRevisionPageSize, cursor } },
	});

	if (listed.error || !listed.data) {
		return { history: { kind: "unavailable" } as DocHistory, restoreForm: emptyForm };
	}

	const { revisions, nextCursor } = listed.data;
	const wanted = url.searchParams.get("revision");
	const index = Math.max(
		0,
		revisions.findIndex((revision) => revision.id === wanted)
	);
	const summary = revisions[index];

	const [chosen, agents] = await Promise.all([
		summary ? readChosen(locals.api, path, summary, revisions[index + 1], nextCursor) : undefined,
		revisions.some((revision) => revision.source === "agent")
			? readAgents(locals.api, workspace.id)
			: Promise.resolve([]),
	]);

	const history: DocHistory = { kind: "ready", doc: read.doc, revisions, nextCursor, agents, chosen };
	const restoreForm = summary
		? await superValidate<RestoreForm, DocFailure>(
				{ revisionId: summary.id, expectedVersion: read.doc.version },
				zod4(restoreDocSchema),
				{ id: restoreFormId, errors: false }
			)
		: emptyForm;

	return { history, restoreForm };
};

async function readChosen(
	api: App.Locals["api"],
	path: { workspaceId: string; docId: string },
	summary: DocRevisionSummary,
	previous: DocRevisionSummary | undefined,
	nextCursor: string | undefined
): Promise<DocChosen> {
	const before = previous ?? (nextCursor ? await olderThan(api, path, nextCursor) : undefined);

	const [revision, prior] = await Promise.all([
		api.GET("/workspaces/{workspaceId}/docs/{docId}/revisions/{revisionId}", {
			params: { path: { ...path, revisionId: summary.id } },
		}),
		before
			? api.GET("/workspaces/{workspaceId}/docs/{docId}/revisions/{revisionId}", {
					params: { path: { ...path, revisionId: before.id } },
				})
			: undefined,
	]);

	if (!revision.data || (prior && !prior.data)) return { kind: "unavailable" };

	return { revision: revision.data, before: prior?.data };
}

async function olderThan(
	api: App.Locals["api"],
	path: { workspaceId: string; docId: string },
	cursor: string
): Promise<DocRevisionSummary | undefined> {
	const { data } = await api.GET("/workspaces/{workspaceId}/docs/{docId}/revisions", {
		params: { path, query: { limit: 1, cursor } },
	});

	return data?.revisions[0];
}

export const actions: Actions = {
	default: async ({ locals, params, request }) => {
		const body = await request.formData();
		const form = await superValidate<RestoreForm, DocFailure>(body, zod4(restoreDocSchema), {
			id: restoreFormId,
		});

		if (!form.valid) return fail(400, { restoreForm: form });

		const workspaceId = String(body.get("workspaceId") ?? "");

		if (!workspaceId) return message(form, { kind: "unavailable" }, { status: 400 });

		const { data, error } = await locals.api.POST(
			"/workspaces/{workspaceId}/docs/{docId}/revisions/{revisionId}/restore",
			{
				params: { path: { workspaceId, docId: params.docId, revisionId: form.data.revisionId } },
				body: { expectedVersion: form.data.expectedVersion },
			}
		);

		if (error || !data) return message(form, readDocFailure(error), { status: 400 });

		redirect(303, docPath(params.workspace, data.id));
	},
};

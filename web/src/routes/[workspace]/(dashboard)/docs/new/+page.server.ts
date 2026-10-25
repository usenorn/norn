import { fail, redirect } from "@sveltejs/kit";
import { message, setError, superValidate, type Infer } from "sveltekit-superforms";
import { zod4 } from "sveltekit-superforms/adapters";
import { newDocFormId, newDocSchema } from "#lib/docs/doc-schema.js";
import { docEditPath, readDocFailure, type DocAudience, type DocFailure } from "#lib/docs/docs.js";
import type { Actions, PageServerLoad } from "./$types";

type NewDocForm = Infer<typeof newDocSchema>;

export const load: PageServerLoad = async ({ parent, url }) => {
	const { tree } = await parent();

	const parentId = url.searchParams.get("parent") ?? "";
	const parentDoc = tree.kind === "ready" ? tree.docs.find((doc) => doc.id === parentId) : undefined;

	const form = await superValidate<NewDocForm, DocFailure>(zod4(newDocSchema), { id: newDocFormId });

	if (parentDoc) {
		form.data.parentId = parentDoc.id;
		form.data.visibility = parentDoc.audience.visibility;
		form.data.projectId = parentDoc.audience.projectId ?? "";
	}

	return { form, parentDoc };
};

export const actions: Actions = {
	default: async ({ locals, params, request }) => {
		const body = await request.formData();
		const form = await superValidate<NewDocForm, DocFailure>(body, zod4(newDocSchema), { id: newDocFormId });

		if (!form.valid) return fail(400, { form });

		const workspaceId = String(body.get("workspaceId") ?? "");

		if (!workspaceId) return message(form, { kind: "unavailable" }, { status: 400 });

		const audience: DocAudience =
			form.data.visibility === "project"
				? { visibility: "project", projectId: form.data.projectId }
				: { visibility: form.data.visibility };

		const { data, error } = await locals.api.POST("/workspaces/{workspaceId}/docs", {
			params: { path: { workspaceId } },
			body: {
				title: form.data.title,
				audience,
				...(form.data.parentId ? { parentId: form.data.parentId } : {}),
			},
		});

		if (error || !data) {
			const failure = readDocFailure(error);

			if (failure.kind === "invalid") {
				if (failure.fields.includes("title")) return setError(form, "title", "Check the title.");
				if (failure.fields.includes("projectId")) return setError(form, "projectId", "Choose a project you can reach.");
				if (failure.fields.includes("parentId")) {
					return message(form, { kind: "not_found" }, { status: 400 });
				}
			}

			return message(form, failure, { status: 400 });
		}

		redirect(303, docEditPath(params.workspace, data.id));
	},
};

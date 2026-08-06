import { fail } from "@sveltejs/kit";
import { message, setError, superValidate, type Infer } from "sveltekit-superforms";
import { zod4 } from "sveltekit-superforms/adapters";
import { keys } from "$lib/api/keys";
import {
	boardFor,
	conflictFailure,
	labelFailureMessage,
	type LabelFailure,
} from "$lib/labels/labels";
import { labelGroupSchema, labelSchema } from "$lib/labels/label-schema";
import type { Actions, PageServerLoad } from "./$types";

type LabelForm = Infer<typeof labelSchema>;
type LabelGroupForm = Infer<typeof labelGroupSchema>;

const labelFormId = "label-form";
const groupFormId = "label-group-form";

export const load: PageServerLoad = async ({ depends, route, locals, parent }) => {
	depends(keys.page(route.id));

	const { workspace, teams } = await parent();
	const path = { workspaceId: workspace.id };

	const [labels, groups] = await Promise.all([
		locals.api.GET("/workspaces/{workspaceId}/labels", { params: { path } }),
		locals.api.GET("/workspaces/{workspaceId}/label-groups", { params: { path } }),
	]);

	return {
		board: boardFor(labels.data, groups.data),
		teams: teams ?? [],
		form: await superValidate<LabelForm, LabelFailure>(zod4(labelSchema), { id: labelFormId }),
		groupForm: await superValidate<LabelGroupForm, LabelFailure>(zod4(labelGroupSchema), {
			id: groupFormId,
		}),
	};
};

function readFailure(error: unknown): LabelFailure {
	if (error && typeof error === "object" && "code" in error) {
		const problem = error as { code: string; issues?: number };
		const conflict = conflictFailure(problem.code, problem.issues);

		if (conflict) return conflict;
	}

	if (error && typeof error === "object" && "status" in error) {
		if ((error as { status: number }).status === 403) return { kind: "forbidden" };
	}

	return { kind: "unavailable" };
}

export const actions: Actions = {
	label: async ({ locals, request }) => {
		const posted = await request.formData();
		const form = await superValidate<LabelForm, LabelFailure>(posted, zod4(labelSchema), {
			id: labelFormId,
		});

		if (!form.valid) return fail(400, { form });

		const workspaceId = String(posted.get("workspaceId") ?? "");
		const labelId = String(posted.get("labelId") ?? "");
		const { name, color, groupId, teamId } = form.data;

		const result = labelId
			? await locals.api.PATCH("/workspaces/{workspaceId}/labels/{labelId}", {
					params: { path: { workspaceId, labelId } },
					body: { name, color, groupId: groupId || null },
				})
			: await locals.api.POST("/workspaces/{workspaceId}/labels", {
					params: { path: { workspaceId } },
					body: {
						name,
						color,
						...(groupId ? { groupId } : {}),
						...(teamId ? { teamId } : {}),
					},
				});

		if (result.data) return { form };

		const failure = readFailure(result.error);

		if (failure.kind === "name_taken") {
			setError(form, "name", labelFailureMessage(failure));

			return fail(400, { form });
		}

		return message(form, failure, { status: 400 });
	},

	group: async ({ locals, request }) => {
		const posted = await request.formData();
		const form = await superValidate<LabelGroupForm, LabelFailure>(posted, zod4(labelGroupSchema), {
			id: groupFormId,
		});

		if (!form.valid) return fail(400, { form });

		const { data, error } = await locals.api.POST("/workspaces/{workspaceId}/label-groups", {
			params: { path: { workspaceId: String(posted.get("workspaceId") ?? "") } },
			body: { name: form.data.name },
		});

		if (data) return { form };

		const failure = readFailure(error);

		if (failure.kind === "group_name_taken") {
			setError(form, "name", labelFailureMessage(failure));

			return fail(400, { form });
		}

		return message(form, failure, { status: 400 });
	},
};

import { fail } from "@sveltejs/kit";
import type { Client } from "openapi-fetch";
import { message, setError, superValidate, type Infer } from "sveltekit-superforms";
import { zod4 } from "sveltekit-superforms/adapters";
import type { paths } from "#lib/api/dashboard.gen.js";
import { keys } from "#lib/api/keys.js";
import { reachInstance } from "#lib/auth/instance.js";
import {
	roadmapFailure,
	roadmapFieldMessage,
	settingsInputOf,
	type RoadmapFailure,
	type RoadmapSettingsOutcome,
	type RoadmapSettingsView,
} from "#lib/roadmap/admin.js";
import { roadmapSettingsSchema } from "#lib/roadmap/roadmap-settings-schema.js";
import type { Actions, PageServerLoad } from "./$types";

type RoadmapSettingsForm = Infer<typeof roadmapSettingsSchema>;

const formId = "roadmap-settings-form";

const failureStatus: Record<RoadmapFailure["kind"], 403 | 404 | 409 | 422 | 500> = {
	stale: 409,
	review_changed: 409,
	duplicate: 409,
	full: 409,
	unreachable: 409,
	invalid: 422,
	gone: 404,
	forbidden: 403,
	unavailable: 500,
};

async function readView(api: Client<paths>, workspaceId: string): Promise<RoadmapSettingsView> {
	const path = { workspaceId };

	const [roadmap, projects] = await Promise.all([
		api.GET("/workspaces/{workspaceId}/roadmap", { params: { path } }),
		api.GET("/workspaces/{workspaceId}/projects", { params: { path } }),
	]);

	if (roadmap.response.status === 403) return { kind: "forbidden" };
	if (!roadmap.data) return { kind: "unavailable" };

	return { kind: "ready", roadmap: roadmap.data, projects: projects.data ?? [] };
}

export const load: PageServerLoad = async ({ depends, route, locals, parent, url }) => {
	const { workspace } = await parent();

	depends(keys.page(route.id), keys.roadmap(workspace.id));

	const [view, instance] = await Promise.all([readView(locals.api, workspace.id), reachInstance(locals.api, url)]);

	const form =
		view.kind === "ready"
			? await superValidate<RoadmapSettingsForm, RoadmapSettingsOutcome>(
					settingsInputOf(view.roadmap),
					zod4(roadmapSettingsSchema),
					{ id: formId, errors: false }
				)
			: await superValidate<RoadmapSettingsForm, RoadmapSettingsOutcome>(zod4(roadmapSettingsSchema), {
					id: formId,
				});

	return { view, selfHosted: instance.selfHosted, form };
};

export const actions: Actions = {
	settings: async ({ locals, request }) => {
		const body = await request.formData();
		const form = await superValidate<RoadmapSettingsForm, RoadmapSettingsOutcome>(
			body,
			zod4(roadmapSettingsSchema),
			{ id: formId }
		);

		if (!form.valid) return fail(400, { form });

		const { expectedVersion, title, description, ...visibility } = form.data;

		const { data, error, response } = await locals.api.PATCH("/workspaces/{workspaceId}/roadmap", {
			params: { path: { workspaceId: String(body.get("workspaceId") ?? "") } },
			body: { expectedVersion, title, description, visibility },
		});

		if (data) {
			form.data = settingsInputOf(data);

			return message(form, { kind: "saved" });
		}

		const failure = roadmapFailure(error, response.status);

		if (failure.kind === "invalid") {
			for (const field of failure.errors) {
				if (field.field === "title" || field.field === "description") {
					setError(form, field.field, roadmapFieldMessage(field));
				}
			}

			return fail(400, { form });
		}

		return message(form, { kind: "failed", failure }, { status: failureStatus[failure.kind] });
	},
};

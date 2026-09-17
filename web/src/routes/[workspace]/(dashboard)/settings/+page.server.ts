import { fail } from "@sveltejs/kit";
import { message, setError, superValidate, type Infer } from "sveltekit-superforms";
import { zod4 } from "sveltekit-superforms/adapters";
import { workspaceSettingsSchema } from "$lib/workspace/settings-schema";
import {
	nameMessage,
	settingsFor,
	slugMessage,
	timezoneMessage,
	weekStartMessage,
	type Workspace,
	type WorkspaceSettings,
} from "$lib/workspace/settings";
import { storageReading } from "$lib/workspace/storage";
import { workspaceSettingsPreviewStates } from "./preview";
import type { Actions, PageServerLoad } from "./$types";

type WorkspaceSettingsForm = Infer<typeof workspaceSettingsSchema>;

const formId = "workspace-settings-form";
const defaultTeamMessage = "That team cannot be the default.";

export const load: PageServerLoad = async ({ locals, parent, url }) => {
	const { workspace, teams } = await parent();

	const storage = await locals.api.GET("/workspaces/{workspaceId}/storage", {
		params: { path: { workspaceId: workspace.id } },
	});

	const preview = import.meta.env.DEV
		? workspaceSettingsPreviewStates[url.searchParams.get("state") ?? ""]
		: undefined;
	const shown = preview && "workspace" in preview.settings ? preview.settings.workspace : workspace;

	const form = await superValidate<WorkspaceSettingsForm, WorkspaceSettings>(
		{
			name: shown.name,
			slug: shown.slug,
			timezone: shown.timezone,
			weekStartsOn: shown.weekStartsOn,
			defaultTeamId: shown.defaultTeamId ?? "",
			...preview?.draft,
		},
		zod4(workspaceSettingsSchema),
		{ id: formId }
	);

	if (preview?.slugError) setError(form, "slug", preview.slugError);

	return {
		settings: settingsFor(workspace),
		storage: storageReading(storage.data),
		workspace,
		teams: (teams ?? []).filter((team) => team.status === "active"),
		form,
	};
};

async function workspaceOf(
	locals: App.Locals,
	workspaceId: string
): Promise<Workspace | undefined> {
	const { data } = await locals.api.GET("/workspaces/{workspaceId}", {
		params: { path: { workspaceId } },
	});

	return data;
}

export const actions: Actions = {
	save: async ({ locals, request }) => {
		const body = await request.formData();
		const form = await superValidate<WorkspaceSettingsForm, WorkspaceSettings>(
			body,
			zod4(workspaceSettingsSchema),
			{ id: formId }
		);

		if (!form.valid) return fail(400, { form });

		const workspaceId = String(body.get("workspaceId") ?? "");
		const previousSlug = String(body.get("previousSlug") ?? "");

		const { data, error, response } = await locals.api.PATCH("/workspaces/{workspaceId}", {
			params: { path: { workspaceId } },
			body: {
				name: form.data.name,
				slug: form.data.slug,
				timezone: form.data.timezone,
				weekStartsOn: form.data.weekStartsOn,
				defaultTeamId: form.data.defaultTeamId || undefined,
			},
		});

		if (data) {
			const renamedFrom = previousSlug && previousSlug !== data.slug ? previousSlug : undefined;

			return message(form, { kind: "saved", workspace: data, renamedFrom });
		}

		if (error?.errors?.length) {
			for (const field of error.errors) {
				if (field.field === "name") setError(form, "name", nameMessage(field.code));
				if (field.field === "slug") setError(form, "slug", slugMessage(field.code));
				if (field.field === "timezone") setError(form, "timezone", timezoneMessage(field.code));
				if (field.field === "weekStartsOn") setError(form, "weekStartsOn", weekStartMessage);
				if (field.field === "defaultTeamId") setError(form, "defaultTeamId", defaultTeamMessage);
			}

			return fail(400, { form });
		}

		if (response.status === 403) return message(form, { kind: "forbidden" }, { status: 403 });

		if (error && "code" in error && error.code === "workspace_deleted") {
			const deleted = await workspaceOf(locals, workspaceId);

			if (deleted) return message(form, settingsFor(deleted), { status: 409 });
		}

		return message(form, { kind: "unavailable" }, { status: 500 });
	},

	delete: async ({ locals, request }) => {
		const workspaceId = String((await request.formData()).get("workspaceId") ?? "");

		const { data, response } = await locals.api.DELETE("/workspaces/{workspaceId}", {
			params: { path: { workspaceId } },
		});

		if (data) return { settings: settingsFor(data) };

		if (response.status === 403) {
			return fail(403, { settings: { kind: "forbidden" } as WorkspaceSettings });
		}

		return fail(500, { settings: { kind: "unavailable" } as WorkspaceSettings });
	},

	restore: async ({ locals, request }) => {
		const workspaceId = String((await request.formData()).get("workspaceId") ?? "");

		const { data, response } = await locals.api.POST("/workspaces/{workspaceId}/restore", {
			params: { path: { workspaceId } },
		});

		if (data) return { settings: settingsFor(data) };

		if (response.status === 403) {
			return fail(403, { settings: { kind: "forbidden" } as WorkspaceSettings });
		}

		return fail(500, { settings: { kind: "unavailable" } as WorkspaceSettings });
	},
};

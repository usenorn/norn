import { fail } from "@sveltejs/kit";
import { message, setError, superValidate, type Infer } from "sveltekit-superforms";
import { zod4 } from "sveltekit-superforms/adapters";
import { workspaceSettingsSchema } from "$lib/workspace/settings-schema";
import {
	nameMessage,
	settingsFor,
	timezoneMessage,
	type Workspace,
	type WorkspaceSettings,
} from "$lib/workspace/settings";
import { storageReading } from "$lib/workspace/storage";
import type { Actions, PageServerLoad } from "./$types";

type WorkspaceSettingsForm = Infer<typeof workspaceSettingsSchema>;

const formId = "workspace-settings-form";
const defaultTeamMessage = "That team cannot be the default.";

export const load: PageServerLoad = async ({ locals, parent }) => {
	const { workspace, teams } = await parent();

	const storage = await locals.api.GET("/workspaces/{workspaceId}/storage", {
		params: { path: { workspaceId: workspace.id } },
	});

	return {
		settings: settingsFor(workspace),
		storage: storageReading(storage.data),
		workspace,
		teams: (teams ?? []).filter((team) => team.status === "active"),
		form: await superValidate<WorkspaceSettingsForm, WorkspaceSettings>(
			{
				name: workspace.name,
				timezone: workspace.timezone,
				defaultTeamId: workspace.defaultTeamId ?? "",
			},
			zod4(workspaceSettingsSchema),
			{ id: formId }
		),
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

		const { data, error } = await locals.api.PATCH("/workspaces/{workspaceId}", {
			params: { path: { workspaceId } },
			body: {
				name: form.data.name,
				timezone: form.data.timezone,
				defaultTeamId: form.data.defaultTeamId || undefined,
			},
		});

		if (data) return message(form, { kind: "saved", workspace: data });

		if (error?.errors?.length) {
			for (const field of error.errors) {
				if (field.field === "name") setError(form, "name", nameMessage(field.code));
				if (field.field === "timezone") setError(form, "timezone", timezoneMessage(field.code));
				if (field.field === "defaultTeamId") setError(form, "defaultTeamId", defaultTeamMessage);
			}

			return fail(400, { form });
		}

		if (error && "code" in error && error.code === "workspace_deleted") {
			const deleted = await workspaceOf(locals, workspaceId);

			if (deleted) return message(form, settingsFor(deleted), { status: 409 });
		}

		return message(form, { kind: "unavailable" }, { status: 500 });
	},

	delete: async ({ locals, request }) => {
		const workspaceId = String((await request.formData()).get("workspaceId") ?? "");

		const { data } = await locals.api.DELETE("/workspaces/{workspaceId}", {
			params: { path: { workspaceId } },
		});

		if (data) return { settings: settingsFor(data) };

		return fail(500, { settings: { kind: "unavailable" } as WorkspaceSettings });
	},

	restore: async ({ locals, request }) => {
		const workspaceId = String((await request.formData()).get("workspaceId") ?? "");

		const { data } = await locals.api.POST("/workspaces/{workspaceId}/restore", {
			params: { path: { workspaceId } },
		});

		if (data) return { settings: settingsFor(data) };

		return fail(500, { settings: { kind: "unavailable" } as WorkspaceSettings });
	},
};

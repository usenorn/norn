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
import type { Actions, PageServerLoad } from "./$types";

type WorkspaceSettingsForm = Infer<typeof workspaceSettingsSchema>;

const formId = "workspace-settings-form";
const defaultTeamMessage = "That team cannot be the default.";

export const load: PageServerLoad = async ({ parent }) => {
	const { workspace, teams } = await parent();

	return {
		settings: settingsFor(workspace),
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

async function unavailableSettings(
	locals: App.Locals,
	workspaceId: string
): Promise<WorkspaceSettings | null> {
	const workspace = await workspaceOf(locals, workspaceId);

	return workspace ? { kind: "unavailable", workspace } : null;
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

		const workspace = await workspaceOf(locals, workspaceId);

		if (!workspace) return fail(500, { form });

		if (error && "code" in error && error.code === "workspace_deleted") {
			return message(form, settingsFor(workspace), { status: 409 });
		}

		return message(form, { kind: "unavailable", workspace }, { status: 500 });
	},

	delete: async ({ locals, request }) => {
		const workspaceId = String((await request.formData()).get("workspaceId") ?? "");

		const { data } = await locals.api.DELETE("/workspaces/{workspaceId}", {
			params: { path: { workspaceId } },
		});

		if (data) return { settings: settingsFor(data) };

		return fail(500, { settings: await unavailableSettings(locals, workspaceId) });
	},

	restore: async ({ locals, request }) => {
		const workspaceId = String((await request.formData()).get("workspaceId") ?? "");

		const { data } = await locals.api.POST("/workspaces/{workspaceId}/restore", {
			params: { path: { workspaceId } },
		});

		if (data) return { settings: settingsFor(data) };

		return fail(500, { settings: await unavailableSettings(locals, workspaceId) });
	},
};

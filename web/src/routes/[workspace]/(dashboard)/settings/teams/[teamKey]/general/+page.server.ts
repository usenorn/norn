import { fail } from "@sveltejs/kit";
import { message, setError, superValidate, type Infer } from "sveltekit-superforms";
import { zod4 } from "sveltekit-superforms/adapters";
import type { TeamSettings } from "$lib/team/team-settings";
import { teamSettingsSchema } from "$lib/team/team-settings-schema";
import { teamNameMessage, type Team } from "$lib/team/teams";
import type { Actions, PageServerLoad } from "./$types";

type TeamSettingsForm = Infer<typeof teamSettingsSchema>;
type TeamPath = { workspaceId: string; teamId: string };

const settingsFormId = "team-settings-form";

export const load: PageServerLoad = async () => {
	const form = await superValidate<TeamSettingsForm, TeamSettings>(zod4(teamSettingsSchema), {
		id: settingsFormId,
	});

	return { form };
};

export const actions: Actions = {
	default: async ({ locals, request }) => {
		const body = await request.formData();
		const form = await superValidate<TeamSettingsForm, TeamSettings>(
			body,
			zod4(teamSettingsSchema),
			{ id: settingsFormId }
		);

		if (!form.valid) return fail(400, { form });

		const path = {
			workspaceId: String(body.get("workspaceId") ?? ""),
			teamId: String(body.get("teamId") ?? ""),
		};

		const { data: updated, error } = await locals.api.PATCH(
			"/workspaces/{workspaceId}/teams/{teamId}",
			{
				params: { path },
				body: {
					name: form.data.name,
					description: form.data.description,
					icon: form.data.icon,
					iconColor: form.data.iconColor,
					estimation: form.data.estimation,
					visibility: form.data.visibility,
				},
			}
		);

		if (updated) return message(form, { kind: "saved", team: updated });

		if (error?.status === 403) {
			const team = await teamAt(locals, path);
			const outcome: TeamSettings = team ? { kind: "read_only", team } : { kind: "unavailable" };

			return message(form, outcome, { status: 403 });
		}

		if (error && "code" in error && error.code === "team_archived") {
			const team = await teamAt(locals, path);
			const outcome: TeamSettings = team ? { kind: "archived", team } : { kind: "unavailable" };

			return message(form, outcome, { status: 409 });
		}

		let handled = false;

		for (const field of error?.errors ?? []) {
			if (field.field === "name") {
				setError(form, "name", teamNameMessage(field.code));
				handled = true;
			}
		}

		if (handled) return fail(400, { form });

		return message(form, { kind: "unavailable" }, { status: 500 });
	},
};

async function teamAt(locals: App.Locals, path: TeamPath): Promise<Team | null> {
	const { data } = await locals.api.GET("/workspaces/{workspaceId}/teams/{teamId}", {
		params: { path },
	});

	return data ?? null;
}

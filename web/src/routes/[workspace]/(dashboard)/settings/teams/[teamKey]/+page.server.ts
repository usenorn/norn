import { fail } from "@sveltejs/kit";
import { message, setError, superValidate, type Infer } from "sveltekit-superforms";
import { zod4 } from "sveltekit-superforms/adapters";
import type { CadenceSetting, CycleCadence } from "$lib/cycles/cycles";
import { keys } from "$lib/api/keys";
import { rosterFor, type TeamRoster } from "$lib/team/members";
import { statesFor, type StateList } from "$lib/team/states";
import { settingsFor, type TeamSettings } from "$lib/team/team-settings";
import { teamSettingsSchema } from "$lib/team/team-settings-schema";
import { teamNameMessage, type Team } from "$lib/team/teams";
import { settingFor, type TriageSetting, type TriageSettings } from "$lib/triage/triage";
import type {
	SourceControlTransitionRule,
	TeamSourceControlSettings,
} from "$lib/source-control/source-control";
import type { AgentSettings } from "$lib/agents/agents";
import type { TeamNotificationSetting } from "$lib/notifications/notifications";
import type { Actions, PageServerLoad } from "./$types";

type TeamSettingsForm = Infer<typeof teamSettingsSchema>;
type TeamPath = { workspaceId: string; teamId: string };

const settingsFormId = "team-settings-form";

export type TeamPageData = {
	settings: TeamSettings;
	roster: TeamRoster;
	states: StateList;
	cadence: CadenceSetting;
	triage: TriageSetting;
	sourceControl: SourceControlTransitionRule[];
	sourceControlSettings: TeamSourceControlSettings;
	agents: AgentSettings | null;
	notifications: TeamNotificationSetting;
};

const unavailable: TeamPageData = {
	settings: { kind: "unavailable" },
	roster: { kind: "unavailable" },
	states: { kind: "unavailable" },
	cadence: { kind: "unavailable" },
	triage: { kind: "unavailable" },
	sourceControl: [],
	sourceControlSettings: { teamId: "", branchTemplate: "{handle}/{reference}-{title}" },
	agents: null,
	notifications: { kind: "unavailable" },
};

export const load: PageServerLoad = async ({ depends, route, locals, params, parent }) => {
	depends(keys.page(route.id));

	const form = await superValidate<TeamSettingsForm, TeamSettings>(zod4(teamSettingsSchema), {
		id: settingsFormId,
	});

	const { workspace, teams } = await parent();

	if (!teams) return { ...unavailable, form };

	const team = teams.find((candidate) => candidate.key === params.teamKey.toUpperCase());

	if (!team) return { ...unavailable, settings: { kind: "not_found" } as TeamSettings, form };

	const path = { workspaceId: workspace.id, teamId: team.id };

	const [members, states, cadence, triage, agents, notifications, sourceControl, scmSettings] =
		await Promise.all([
		locals.api.GET("/workspaces/{workspaceId}/teams/{teamId}/members", { params: { path } }),
		locals.api.GET("/workspaces/{workspaceId}/teams/{teamId}/states", { params: { path } }),
		locals.api.GET("/workspaces/{workspaceId}/teams/{teamId}/cycle-cadence", { params: { path } }),
		locals.api.GET("/workspaces/{workspaceId}/teams/{teamId}/triage", { params: { path } }),
		locals.api.GET("/workspaces/{workspaceId}/teams/{teamId}/agent-settings", { params: { path } }),
		locals.api.GET("/workspaces/{workspaceId}/teams/{teamId}/notification-settings", {
			params: { path },
		}),
		locals.api.GET("/workspaces/{workspaceId}/teams/{teamId}/source-control/rules", {
			params: { path },
		}),
		locals.api.GET("/workspaces/{workspaceId}/teams/{teamId}/source-control/settings", {
			params: { path },
		}),
	]);

	return {
		settings: settingsFor(team),
		roster: rosterFor(members.data),
		states: statesFor(states.data),
		cadence: cadenceFor(cadence.data, cadence.response.status),
		triage: settingFor(triage.data, triage.response.status),
		sourceControl: (sourceControl.data ?? []) as SourceControlTransitionRule[],
		sourceControlSettings: scmSettings.data ?? {
			teamId: team.id,
			branchTemplate: "{handle}/{reference}-{title}",
		},
		agents: agents.data ?? null,
		notifications: (notifications.data
			? { kind: "ready", settings: notifications.data }
			: { kind: "unavailable" }) as TeamNotificationSetting,
		form,
	};
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
				body: { name: form.data.name, visibility: form.data.visibility },
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

function cadenceFor(cadence: CycleCadence | undefined, status: number): CadenceSetting {
	if (cadence) return { kind: "enabled", cadence };
	if (status === 404) return { kind: "disabled" };

	return { kind: "unavailable" };
}

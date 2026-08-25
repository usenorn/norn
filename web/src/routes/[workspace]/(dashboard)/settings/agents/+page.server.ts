import { fail } from "@sveltejs/kit";
import { message, setError, superValidate, type Infer } from "sveltekit-superforms";
import { zod4 } from "sveltekit-superforms/adapters";
import { keys } from "$lib/api/keys";
import { registerAgentSchema } from "$lib/agents/register-agent-schema";
import {
	failureMessage,
	registerFailure,
	type Agent,
	type AgentFailure,
	type AgentListing,
} from "$lib/agents/agents";
import type { Actions, PageServerLoad } from "./$types";

type RegisterForm = Infer<typeof registerAgentSchema>;

const formId = "register-agent-form";

type AgentOutcome = { kind: "issued"; agent: Agent; value: string } | AgentFailure;

export const load: PageServerLoad = async ({ depends, route, locals, parent }) => {
	depends(keys.page(route.id));

	const { workspace, teams } = await parent();
	depends(keys.agents(workspace.id));
	const form = await superValidate<RegisterForm, AgentOutcome>(zod4(registerAgentSchema), {
		id: formId,
	});

	const agents = await locals.api.GET("/workspaces/{workspaceId}/agents", {
		params: { path: { workspaceId: workspace.id } },
	});
	const reachable = (teams ?? []).filter((team) => team.status === "active");

	if (agents.error) {
		return {
			form,
			teams: reachable,
			listing: {
				kind:
					agents.error.status === 403
						? "forbidden"
						: agents.error.status === 409
							? "authority_missing"
							: "unavailable",
			} as AgentListing,
		};
	}

	if (!agents.data || agents.data.length === 0) {
		return { form, teams: reachable, listing: { kind: "empty" } as AgentListing };
	}

	return {
		form,
		teams: reachable,
		listing: { kind: "ready", agents: agents.data } as AgentListing,
	};
};

export const actions: Actions = {
	default: async ({ locals, request }) => {
		const body = await request.formData();
		const form = await superValidate<RegisterForm, AgentOutcome>(body, zod4(registerAgentSchema), {
			id: formId,
		});

		if (!form.valid) return fail(400, { form });

		const workspaceId = String(body.get("workspaceId") ?? "");

		const { data, error } = await locals.api.POST("/workspaces/{workspaceId}/agents", {
			params: { path: { workspaceId } },
			body: {
				name: form.data.name,
				icon: form.data.icon,
				scopes: form.data.scopes,
				allTeams: form.data.allTeams,
				teamIds: form.data.allTeams ? undefined : form.data.teamIds,
				actionLimit: form.data.actionLimit,
			},
		});

		if (error) {
			let handled = false;

			for (const field of error.errors ?? []) {
				if (field.field === "name") {
					setError(
						form,
						"name",
						field.code === "too_long" ? "Keep the name under 80 characters." : "Name this agent."
					);
				} else if (field.field === "icon") {
					setError(form, "icon", "Choose a supported icon.");
				} else if (field.field === "actionLimit") {
					setError(form, "actionLimit", "Choose between 1 and 6000 actions a minute.");
				} else {
					continue;
				}

				handled = true;
			}

			if (handled) return fail(400, { form });

			const failure = registerFailure(error);

			if (failure.kind === "name_taken") {
				setError(form, "name", failureMessage(failure));

				return fail(400, { form });
			}

			return message(form, failure, { status: 400 });
		}

		if (!data) return message(form, { kind: "unavailable" }, { status: 500 });

		form.data.name = "";
		form.data.scopes = [];
		form.data.teamIds = [];
		form.data.allTeams = true;

		return message(form, { kind: "issued", agent: data.agent, value: data.value });
	},
};

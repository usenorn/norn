import { fail } from "@sveltejs/kit";
import { message, setError, superValidate, type Infer } from "sveltekit-superforms";
import { zod4 } from "sveltekit-superforms/adapters";
import { keys } from "$lib/api/keys";
import { mintTokenSchema } from "$lib/account/mint-token-schema";
import {
	defaultExpiryDays,
	failureMessage,
	mintFailure,
	type MintOutcome,
	type TokenListing,
} from "$lib/account/tokens";
import type { Team } from "$lib/team/teams";
import type { Actions, PageServerLoad } from "./$types";

type MintTokenForm = Infer<typeof mintTokenSchema>;

const dayInMs = 86_400_000;

export const load: PageServerLoad = async ({ depends, route, locals, parent }) => {
	depends(keys.page(route.id));

	const { workspaces } = await parent();

	const [tokens, ...rosters] = await Promise.all([
		locals.api.GET("/tokens"),
		...workspaces.map((workspace) =>
			locals.api.GET("/workspaces/{workspaceId}/teams", {
				params: { path: { workspaceId: workspace.id } },
			})
		),
	]);

	const teams: Record<string, Team[]> = {};

	workspaces.forEach((workspace, index) => {
		teams[workspace.id] = rosters[index]?.data ?? [];
	});

	const form = await superValidate<MintTokenForm, MintOutcome>(zod4(mintTokenSchema));

	if (tokens.error) {
		return {
			teams,
			form,
			listing: {
				kind: tokens.error.status === 403 ? "forbidden" : "unavailable",
			} as TokenListing,
		};
	}

	if (!tokens.data || tokens.data.length === 0) {
		return { teams, form, listing: { kind: "empty" } as TokenListing };
	}

	return { teams, form, listing: { kind: "ready", tokens: tokens.data } as TokenListing };
};

export const actions: Actions = {
	default: async ({ locals, request }) => {
		const form = await superValidate<MintTokenForm, MintOutcome>(request, zod4(mintTokenSchema));

		if (!form.valid) return fail(400, { form });

		const minting = await locals.api
			.POST("/tokens", {
				body: {
					name: form.data.name,
					scopes: form.data.scopes,
					grants: form.data.grants.map((grant) => ({
						workspaceId: grant.workspaceId,
						allTeams: grant.allTeams,
						teamIds: grant.allTeams ? undefined : grant.teamIds,
					})),
					expiresAt: new Date(Date.now() + form.data.expiresInDays * dayInMs).toISOString(),
				},
			})
			.catch(() => null);

		if (!minting) return message(form, { kind: "unavailable" }, { status: 500 });

		if (minting.error) {
			const failure = mintFailure(minting.error);

			if (failure.kind === "name_taken") {
				setError(form, "name", failureMessage(failure));

				return fail(400, { form });
			}

			return message(form, failure, { status: 400 });
		}

		if (!minting.data) return message(form, { kind: "unavailable" }, { status: 500 });

		form.data = { name: "", scopes: [], grants: [], expiresInDays: defaultExpiryDays };

		return message(form, { kind: "minted", token: minting.data.token, value: minting.data.value });
	},
};

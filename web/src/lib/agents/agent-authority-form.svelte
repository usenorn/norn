<script lang="ts">
	import { defaults, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import { Button } from "#lib/components/ui/button/index.js";
	import type { Team } from "#lib/team/teams.js";
	import AgentPermissionPicker from "./agent-permission-picker.svelte";
	import AgentTeamReach from "./agent-team-reach.svelte";
	import { agentAuthoritySchema, type AgentAuthorityInput } from "./agent-authority-schema";
	import { agentPermissions, type AgentPermission, type APIScope } from "./agents";

	let {
		scopes,
		allTeams,
		teamIds,
		teams,
		grantable,
		locked = false,
		onsave,
	}: {
		scopes: APIScope[];
		allTeams: boolean;
		teamIds: string[];
		teams: Team[];
		grantable: APIScope[] | null;
		locked?: boolean;
		onsave: (input: AgentAuthorityInput) => Promise<boolean>;
	} = $props();

	// svelte-ignore state_referenced_locally
	const opened: AgentAuthorityInput = {
		scopes: scopes.filter((scope): scope is AgentPermission =>
			(agentPermissions as readonly APIScope[]).includes(scope)
		),
		allTeams,
		teamIds: allTeams ? [] : teamIds,
	};

	let held = $state(opened);

	const form = superForm(defaults(opened, zod4(agentAuthoritySchema)), {
		id: "agent-authority-form",
		SPA: true,
		validators: zod4Client(agentAuthoritySchema),
		resetForm: false,
		invalidateAll: false,
		onUpdate: async ({ form: pending }) => {
			if (!pending.valid) return;

			if (await onsave(pending.data)) held = { ...pending.data };
		},
	});

	const { form: formData, enhance, submitting } = form;

	const busy = $derived(locked || $submitting);

	const dirty = $derived(
		!sameMembers($formData.scopes, held.scopes) ||
			$formData.allTeams !== held.allTeams ||
			!sameMembers($formData.teamIds, held.teamIds)
	);

	function sameMembers(left: readonly string[], right: readonly string[]): boolean {
		return left.length === right.length && left.every((entry) => right.includes(entry));
	}
</script>

<form
	id="agent-authority-form"
	method="POST"
	use:enhance
	class="flex flex-col gap-4 rounded-lg border border-line-strong p-4"
>
	<AgentPermissionPicker {form} {grantable} {busy} />

	<AgentTeamReach {form} {teams} {busy} />

	<div class="flex justify-end">
		<Button type="submit" size="sm" disabled={busy || !dirty}>
			{$submitting ? "Saving…" : "Save permissions"}
		</Button>
	</div>
</form>

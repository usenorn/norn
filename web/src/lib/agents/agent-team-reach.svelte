<script lang="ts" generics="T extends AgentAuthorityInput">
	import type { FormPath, SuperForm } from "sveltekit-superforms";
	import * as Form from "#lib/components/ui/form/index.js";
	import { Checkbox } from "#lib/components/ui/checkbox/index.js";
	import type { Team } from "#lib/team/teams.js";
	import type { AgentAuthorityInput } from "./agent-authority-schema";

	let {
		form,
		teams,
		busy,
		inline = false,
	}: {
		form: SuperForm<T>;
		teams: Team[];
		busy: boolean;
		inline?: boolean;
	} = $props();

	// svelte-ignore state_referenced_locally
	const { form: formData } = form;

	function reachEveryTeam(checked: boolean) {
		formData.update((entered) => ({ ...entered, allTeams: checked, teamIds: [] }));
	}

	function toggleTeam(teamId: string, checked: boolean) {
		const next = new Set($formData.teamIds);

		if (checked) next.add(teamId);
		else next.delete(teamId);

		formData.update((entered) => ({ ...entered, teamIds: [...next] }));
	}
</script>

<Form.Fieldset {form} name={"allTeams" as FormPath<T>}>
	<Form.Legend>Team access</Form.Legend>
	<div class="flex flex-col gap-2">
		<div class="flex items-center gap-2">
			{#if inline}
				<input
					id="agent-all-teams"
					type="checkbox"
					name="allTeams"
					value="true"
					disabled={busy}
					checked={$formData.allTeams}
					onchange={(event) => reachEveryTeam(event.currentTarget.checked)}
					class="size-3.5 accent-ink-900"
				/>
			{:else}
				<Checkbox
					id="agent-all-teams"
					disabled={busy}
					checked={$formData.allTeams}
					onCheckedChange={(checked) => reachEveryTeam(checked === true)}
				/>
			{/if}
			<label for="agent-all-teams" class="text-sm leading-normal text-ink-600">
				Every team you can see
			</label>
		</div>

		{#if inline || !$formData.allTeams}
			{#if teams.length === 0}
				<p class="text-xs text-muted-foreground">No teams are available.</p>
			{:else}
				<div class="grid gap-1.5 pl-6 sm:grid-cols-2">
					{#each teams as team (team.id)}
						<div class="flex items-center gap-2">
							{#if inline}
								<input
									id={`agent-team-${team.id}`}
									type="checkbox"
									name="teamIds"
									value={team.id}
									disabled={busy}
									checked={$formData.teamIds.includes(team.id)}
									onchange={(event) => toggleTeam(team.id, event.currentTarget.checked)}
									class="size-3.5 accent-ink-900"
								/>
							{:else}
								<Checkbox
									id={`agent-team-${team.id}`}
									disabled={busy}
									checked={$formData.teamIds.includes(team.id)}
									onCheckedChange={(checked) => toggleTeam(team.id, checked === true)}
								/>
							{/if}
							<label for={`agent-team-${team.id}`} class="text-sm text-ink-600">
								{team.name}
							</label>
						</div>
					{/each}
				</div>
			{/if}
		{/if}
	</div>
	<Form.FieldErrors />
</Form.Fieldset>

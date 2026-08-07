<script lang="ts">
	import CircleX from "@lucide/svelte/icons/circle-x";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";

	import { api } from "$lib/api";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import { Label } from "$lib/components/ui/label/index.js";
	import {
		changeStateLabel,
		changeStateOrder,
		failureMessage,
		sourceControlFailure,
		type CodeChangeState,
		type SourceControlFailure,
		type SourceControlTransitionRule,
	} from "./source-control";

	let {
		workspace,
		team,
		rules,
		states,
		locked = false,
	}: {
		workspace: { id: string };
		team: { id: string };
		rules: SourceControlTransitionRule[];
		states: { id: string; name: string }[];
		locked?: boolean;
	} = $props();

	let saved = $state.raw<SourceControlTransitionRule[] | null>(null);
	let failure = $state<SourceControlFailure | null>(null);
	let working = $state("");

	const current = $derived(saved ?? rules);
	const disabled = $derived(locked || working !== "");

	function ruleFor(trigger: CodeChangeState): SourceControlTransitionRule | undefined {
		return current.find((rule) => rule.trigger === trigger);
	}

	async function choose(trigger: CodeChangeState, stateId: string) {
		working = trigger;
		failure = null;

		const path = { workspaceId: workspace.id, teamId: team.id };

		const { data, error } = stateId
			? await api.PUT("/workspaces/{workspaceId}/teams/{teamId}/source-control/rules", {
					params: { path },
					body: { trigger, stateId },
				})
			: await api.DELETE(
					"/workspaces/{workspaceId}/teams/{teamId}/source-control/rules/{trigger}",
					{ params: { path: { ...path, trigger } } },
				);

		working = "";

		if (error) {
			failure = sourceControlFailure(error);

			return;
		}

		if (data) saved = data;
	}
</script>

<div class="flex flex-col gap-3">
	<p class="text-sm leading-normal text-muted-foreground text-pretty">
		When a change linked to one of this team's issues reaches a state on the platform, the issue
		moves to the state you name here. An issue somebody is editing at that moment, or one that
		still has open sub-issues, is left where it is and the link is recorded either way.
	</p>

	<div class="flex flex-col gap-3">
		{#each changeStateOrder as trigger (trigger)}
			{@const rule = ruleFor(trigger)}
			<div class="flex flex-col gap-1">
				<Label for="scm-rule-{trigger}">{changeStateLabel(trigger)}</Label>
				<select
					id="scm-rule-{trigger}"
					{disabled}
					value={rule?.stateId ?? ""}
					onchange={(event) => choose(trigger, event.currentTarget.value)}
					class="h-9 max-w-sm rounded-md border border-line-subtle bg-transparent px-3 text-sm text-ink-900"
				>
					<option value="">Leave the issue alone</option>
					{#each states as state (state.id)}
						<option value={state.id}>{state.name}</option>
					{/each}
				</select>

				{#if rule && !rule.stateName}
					<p class="flex items-start gap-1.5 pt-1 text-sm text-destructive">
						<TriangleAlert class="mt-0.5 size-icon-row shrink-0" aria-hidden="true" />
						The state this pointed at no longer exists, so nothing is being moved. Choose another.
					</p>
				{/if}
			</div>
		{/each}
	</div>

	{#if failure}
		<Alert.Root variant="destructive">
			<CircleX aria-hidden="true" />
			<Alert.Title>That did not stick</Alert.Title>
			<Alert.Description>{failureMessage(failure)}</Alert.Description>
		</Alert.Root>
	{/if}
</div>

<script lang="ts">
	import CircleX from "@lucide/svelte/icons/circle-x";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";

	import { api } from "$lib/api";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import { Checkbox } from "$lib/components/ui/checkbox/index.js";
	import { Label } from "$lib/components/ui/label/index.js";
	import {
		failureMessage,
		sourceControlFailure,
		type SourceControlFailure,
		type TeamSourceControlSettings,
	} from "./source-control";

	let {
		workspace,
		team,
		settings,
		states,
		locked = false,
	}: {
		workspace: { id: string };
		team: { id: string };
		settings: TeamSourceControlSettings;
		states: { id: string; name: string }[];
		locked?: boolean;
	} = $props();

	let saved = $state.raw<TeamSourceControlSettings | null>(null);
	let failure = $state<SourceControlFailure | null>(null);
	let working = $state(false);

	const current = $derived(saved ?? settings);
	const disabled = $derived(locked || working);

	async function save(advanceOnMerge: boolean, mergedStateId: string) {
		working = true;
		failure = null;

		const { data, error } = await api.PUT(
			"/workspaces/{workspaceId}/teams/{teamId}/source-control",
			{
				params: { path: { workspaceId: workspace.id, teamId: team.id } },
				body: {
					advanceOnMerge,
					mergedStateId: mergedStateId || undefined,
					clearState: mergedStateId === "",
				},
			},
		);

		working = false;

		if (error) {
			failure = sourceControlFailure(error);

			return;
		}

		if (data) saved = data;
	}
</script>

<div class="flex flex-col gap-3">
	<div class="flex items-start gap-2.5">
		<Checkbox
			id="advance-on-merge"
			checked={current.advanceOnMerge}
			{disabled}
			onCheckedChange={(checked) => save(checked === true, current.mergedStateId ?? "")}
		/>
		<div class="flex flex-col gap-0.5">
			<Label for="advance-on-merge">Move an issue on when a linked change merges</Label>
			<p class="text-sm leading-normal text-muted-foreground text-pretty">
				Only a merged pull or merge request does this. An issue somebody is editing at that
				moment, or one that still has open sub-issues, is left where it is and the link is
				recorded either way.
			</p>
		</div>
	</div>

	{#if current.advanceOnMerge}
		<div class="flex flex-col gap-1 pl-7">
			<Label for="merged-state">Move it to</Label>
			<select
				id="merged-state"
				{disabled}
				value={current.mergedStateId ?? ""}
				onchange={(event) => save(true, event.currentTarget.value)}
				class="h-9 max-w-sm rounded-md border border-line-subtle bg-transparent px-3 text-sm text-ink-900"
			>
				<option value="">This team's completion state</option>
				{#each states as state (state.id)}
					<option value={state.id}>{state.name}</option>
				{/each}
			</select>

			{#if !current.targetResolved}
				<p class="flex items-start gap-1.5 pt-1 text-sm text-destructive">
					<TriangleAlert class="mt-0.5 size-icon-row shrink-0" aria-hidden="true" />
					The state this pointed at no longer exists, so nothing is being moved. Choose another.
				</p>
			{:else if current.targetName}
				<p class="pt-1 text-sm text-muted-foreground">
					A merged change moves the issue to {current.targetName}.
				</p>
			{/if}
		</div>
	{/if}

	{#if failure}
		<Alert.Root variant="destructive">
			<CircleX aria-hidden="true" />
			<Alert.Title>That did not stick</Alert.Title>
			<Alert.Description>{failureMessage(failure)}</Alert.Description>
		</Alert.Root>
	{/if}
</div>

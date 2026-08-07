<script lang="ts">
	import CircleX from "@lucide/svelte/icons/circle-x";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";

	import { api } from "$lib/api";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { Label } from "$lib/components/ui/label/index.js";
	import {
		changeStateLabel,
		changeStateOrder,
		failureMessage,
		sourceControlFailure,
		type CodeChangeState,
		type SourceControlFailure,
		type SourceControlTransitionRule,
		type TeamSourceControlSettings,
	} from "./source-control";

	let {
		workspace,
		team,
		rules,
		settings,
		states,
		locked = false,
	}: {
		workspace: { id: string };
		team: { id: string };
		rules: SourceControlTransitionRule[];
		settings: TeamSourceControlSettings;
		states: { id: string; name: string }[];
		locked?: boolean;
	} = $props();

	let saved = $state.raw<SourceControlTransitionRule[] | null>(null);
	let storedTemplate = $state.raw<string | null>(null);
	let edited = $state.raw<string | null>(null);
	let templateError = $state("");
	let savingTemplate = $state(false);
	let failure = $state<SourceControlFailure | null>(null);
	let working = $state("");

	const current = $derived(saved ?? rules);
	const currentTemplate = $derived(storedTemplate ?? settings.branchTemplate);
	const template = $derived(edited ?? currentTemplate);
	const disabled = $derived(locked || working !== "");

	function ruleFor(trigger: CodeChangeState): SourceControlTransitionRule | undefined {
		return current.find((rule) => rule.trigger === trigger);
	}

	async function saveTemplate() {
		savingTemplate = true;
		templateError = "";

		const { data, error } = await api.PUT(
			"/workspaces/{workspaceId}/teams/{teamId}/source-control/settings",
			{
				params: { path: { workspaceId: workspace.id, teamId: team.id } },
				body: { branchTemplate: template },
			},
		);

		savingTemplate = false;

		if (error) {
			templateError =
				error.detail?.trim() ||
				"A template has to contain {reference}, or the branch names no issue.";

			return;
		}

		if (data) {
			storedTemplate = data.branchTemplate;
			edited = null;
		}
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

	<div class="flex flex-col gap-1 border-t border-line-subtle pt-3">
		<Label for="branch-template">Branch name</Label>
		<p class="text-sm leading-normal text-muted-foreground text-pretty">
			What Norn offers on an issue for somebody to copy before starting work. Use
			<code>{"{handle}"}</code>, <code>{"{reference}"}</code> and <code>{"{title}"}</code>.
			The reference is what creates the link, so a template without it is refused.
		</p>
		<div class="flex flex-wrap items-start gap-2">
			<Input
				id="branch-template"
				value={template}
				oninput={(event) => (edited = event.currentTarget.value)}
				disabled={locked || savingTemplate}
				class="max-w-sm"
			/>
			<Button
				variant="secondary"
				onclick={saveTemplate}
				disabled={locked || savingTemplate || template === currentTemplate}
			>
				{savingTemplate ? "Saving…" : "Save"}
			</Button>
		</div>
		{#if templateError}
			<p class="pt-1 text-sm text-destructive">{templateError}</p>
		{/if}
	</div>

	{#if failure}
		<Alert.Root variant="destructive">
			<CircleX aria-hidden="true" />
			<Alert.Title>That did not stick</Alert.Title>
			<Alert.Description>{failureMessage(failure)}</Alert.Description>
		</Alert.Root>
	{/if}
</div>

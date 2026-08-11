<script lang="ts">
	import { invalidate } from "$app/navigation";
	import { page } from "$app/state";
	import { keys } from "$lib/api/keys";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Select from "$lib/components/ui/select/index.js";
	import { api } from "$lib/api";
	import {
		approvalsPath,
		holdLabels,
		holdOptionHints,
		holdOptionLabels,
		type AgentHold,
		type AgentSettings,
	} from "$lib/agents/agents";
	import type { Team } from "$lib/team/teams";

	let {
		workspace,
		team,
		settings,
		locked = false,
	}: {
		workspace: { id: string; slug: string };
		team: Team;
		settings: AgentSettings | null;
		locked?: boolean;
	} = $props();

	let saved = $state<AgentSettings | null>(null);
	let failed = $state(false);
	let working = $state(false);

	const current = $derived<AgentSettings>(
		saved ??
			settings ?? {
				holdComments: "never",
				holdStateChanges: "never",
				holdIssueEdits: "never",
				holdIssueCreation: "never",
			}
	);
	const disabled = $derived(locked || working);
	const holding = $derived(holdLabels.some((rule) => current[rule.key] !== "never"));

	async function save(next: Partial<AgentSettings>) {
		working = true;
		failed = false;

		try {
			const { data, error } = await api.PUT(
				"/workspaces/{workspaceId}/teams/{teamId}/agent-settings",
				{
					params: { path: { workspaceId: workspace.id, teamId: team.id } },
					body: { ...current, ...next },
				}
			);

			if (error || !data) {
				failed = true;

				return;
			}

			saved = data;
			await invalidate(keys.page(page.route.id));
		} catch {
			failed = true;
		} finally {
			working = false;
		}
	}
</script>

<section class="flex flex-col gap-4">
	<div class="flex flex-col gap-1">
		<h2 class="text-md font-medium tracking-snug text-ink-900">Agents</h2>
		<p class="text-sm leading-normal text-muted-foreground text-pretty">
			Hold what an agent does in {team.name} until a person agrees to it. Approving applies the
			change as the agent, so the record still says which one did it.
		</p>
	</div>

	{#if failed}
		<Alert.Root variant="destructive">
			<CircleX aria-hidden="true" />
			<Alert.Title>That did not stick</Alert.Title>
			<Alert.Description>We could not save it. Try again in a moment.</Alert.Description>
		</Alert.Root>
	{/if}

	{#if settings === null}
		<p class="text-sm leading-normal text-muted-foreground">
			We could not read this team's agent settings.
		</p>
	{:else}
		<div class="flex flex-col gap-3 rounded-lg border border-line-default p-4">
			{#each holdLabels as rule (rule.key)}
				<div class="flex flex-col gap-1.5">
					<div class="flex flex-wrap items-center justify-between gap-2">
						<span class="text-sm leading-normal text-ink-900">{rule.title}</span>
						<Select.Root
							type="single"
							value={current[rule.key]}
							{disabled}
							onValueChange={(value) => save({ [rule.key]: value as AgentHold })}
						>
							<Select.Trigger class="w-48" aria-label="{rule.title}: when to hold">
								{holdOptionLabels[current[rule.key]] ?? "Never hold"}
							</Select.Trigger>
							<Select.Content>
								{#each rule.options as option (option)}
									<Select.Item value={option} label={holdOptionLabels[option]}>
										{holdOptionLabels[option]}
									</Select.Item>
								{/each}
							</Select.Content>
						</Select.Root>
					</div>
					<span class="text-xs leading-normal text-muted-foreground text-pretty">
						{rule.detail}
					</span>
					<span class="text-xs leading-normal text-muted-foreground text-pretty">
						{holdOptionHints[current[rule.key]] ?? holdOptionHints.never}
					</span>
				</div>
			{/each}
		</div>

		{#if holding}
			<p class="text-sm leading-normal text-muted-foreground text-pretty">
				Held actions wait on the
				<a
					href={approvalsPath(workspace.slug)}
					class="text-ink-900 underline underline-offset-2"
				>
					approvals screen
				</a>
				until somebody decides.
			</p>
		{:else}
			<p class="text-sm leading-normal text-muted-foreground text-pretty">
				Nothing is held. An agent with the right permissions acts in {team.name} straight away.
			</p>
		{/if}
	{/if}
</section>

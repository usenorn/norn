<script lang="ts">
	import { invalidateAll } from "$app/navigation";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Checkbox } from "$lib/components/ui/checkbox/index.js";
	import { Label } from "$lib/components/ui/label/index.js";
	import { api } from "$lib/api";
	import {
		readTriageFailure,
		triageFailureMessage,
		type TriageFailure,
		type TriageSetting,
		type TriageSettings,
	} from "$lib/triage/triage";
	import type { Team } from "$lib/team/teams";

	let {
		workspace,
		team,
		setting,
		locked = false,
	}: {
		workspace: { id: string; slug: string };
		team: Team;
		setting: TriageSetting;
		locked?: boolean;
	} = $props();

	let saved = $state<TriageSetting | null>(null);
	let failure = $state<TriageFailure | null>(null);
	let working = $state(false);

	const current = $derived<TriageSetting>(saved ?? setting);
	const on = $derived(current.kind === "on");
	const rules = $derived(current.kind === "on" ? current.settings : null);
	const disabled = $derived(locked || working);

	const routes: { key: keyof TriageSettings; label: string; note: string }[] = [
		{
			key: "routeIntegrations",
			label: "Filed by an integration",
			note: "Anything arriving through an API token waits for someone to look at it.",
		},
		{
			key: "routeAgents",
			label: "Filed by an agent",
			note: "An agent cannot fill this team's backlog without a person agreeing first.",
		},
		{
			key: "routeNonMembers",
			label: "Filed by someone not on this team",
			note: "Work handed over from elsewhere in the workspace waits here first.",
		},
	];

	async function save(next: Partial<TriageSettings>) {
		working = true;
		failure = null;

		const body = {
			routeAgents: rules?.routeAgents ?? true,
			routeIntegrations: rules?.routeIntegrations ?? true,
			routeNonMembers: rules?.routeNonMembers ?? false,
			...next,
		};

		try {
			const { data, error } = await api.PUT(
				"/workspaces/{workspaceId}/teams/{teamId}/triage",
				{ params: { path: { workspaceId: workspace.id, teamId: team.id } }, body }
			);

			if (error || !data) {
				failure = readTriageFailure(error);

				return;
			}

			saved = { kind: "on", settings: data };
			await invalidateAll();
		} catch {
			failure = { kind: "unavailable" };
		} finally {
			working = false;
		}
	}

	async function stop() {
		working = true;
		failure = null;

		try {
			const { error } = await api.DELETE("/workspaces/{workspaceId}/teams/{teamId}/triage", {
				params: { path: { workspaceId: workspace.id, teamId: team.id } },
			});

			if (error) {
				failure = readTriageFailure(error);

				return;
			}

			saved = { kind: "off" };
			await invalidateAll();
		} catch {
			failure = { kind: "unavailable" };
		} finally {
			working = false;
		}
	}
</script>

<section class="flex flex-col gap-4">
	<div class="flex flex-col gap-1">
		<h2 class="text-md font-medium tracking-snug text-ink-900">Triage</h2>
		<p class="text-sm leading-normal text-muted-foreground text-pretty">
			Hold incoming issues for review instead of dropping them straight into the backlog, so what
			the team is counting is work somebody agreed to.
		</p>
	</div>

	{#if failure}
		<Alert.Root variant="destructive">
			<CircleX aria-hidden="true" />
			<Alert.Title>That did not stick</Alert.Title>
			<Alert.Description>{triageFailureMessage(failure)}</Alert.Description>
		</Alert.Root>
	{/if}

	{#if current.kind === "loading"}
		<div class="h-24 animate-pulse rounded-lg bg-paper-2" aria-busy="true"></div>
	{:else if current.kind === "unavailable"}
		<p class="text-sm leading-normal text-muted-foreground">
			We could not read this team's triage settings.
		</p>
	{:else if !on}
		<div class="flex flex-col items-start gap-3 rounded-lg border border-line-default p-4">
			<p class="text-md leading-normal text-muted-foreground text-pretty">
				Everything filed into {team.name} goes straight to its backlog.
			</p>
			<Button
				variant="secondary"
				size="sm"
				{disabled}
				onclick={() => save({ routeAgents: true, routeIntegrations: true, routeNonMembers: false })}
			>
				{working ? "Turning on" : "Hold incoming issues for review"}
			</Button>
		</div>
	{:else}
		<div class="flex flex-col gap-3 rounded-lg border border-line-default p-4">
			{#each routes as route (route.key)}
				<div class="flex items-start gap-3">
					<Checkbox
						id="triage-{route.key}"
						checked={Boolean(rules?.[route.key])}
						{disabled}
						onCheckedChange={(checked) => save({ [route.key]: checked === true })}
					/>
					<div class="flex min-w-0 flex-col gap-0.5">
						<Label for="triage-{route.key}" class="text-md text-ink-900">{route.label}</Label>
						<p class="text-sm leading-normal text-muted-foreground text-pretty">{route.note}</p>
					</div>
				</div>
			{/each}
		</div>
		<div class="flex flex-wrap gap-2">
			<Button variant="ghost" size="sm" {disabled} onclick={stop}>
				{working ? "Working" : "Stop holding anything"}
			</Button>
		</div>
	{/if}
</section>

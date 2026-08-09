<script lang="ts">
	import { invalidate } from "$app/navigation";
	import { page } from "$app/state";
	import { keys } from "$lib/api/keys";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Select from "$lib/components/ui/select/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { api } from "$lib/api";
	import {
		cycleFailureMessage,
		cycleLengths,
		lengthLabel,
		readCycleFailure,
		teamCyclesPath,
		weekdayLabel,
		weekdays,
		type CadenceSetting,
		type CycleFailure,
	} from "$lib/cycles/cycles";
	import { cycleWindow } from "$lib/time";
	import type { Team } from "$lib/team/teams";

	let {
		workspace,
		team,
		setting,
		locked = false,
	}: {
		workspace: { id: string; slug: string };
		team: Team;
		setting: CadenceSetting;
		locked?: boolean;
	} = $props();

	let saved = $state<CadenceSetting | null>(null);
	let failure = $state<CycleFailure | null>(null);
	let working = $state(false);

	const current = $derived<CadenceSetting>(saved ?? setting);
	const enabled = $derived(current.kind === "enabled");
	const cadence = $derived(current.kind === "enabled" ? current.cadence : null);
	const disabled = $derived(locked || working);

	let length = $state(2);
	let weekday = $state(1);

	$effect(() => {
		if (cadence) {
			length = cadence.lengthWeeks;
			weekday = cadence.startsOn;
		}
	});

	async function save(lengthWeeks: number, startsOn: number) {
		working = true;
		failure = null;

		try {
			const { data, error } = await api.PUT(
				"/workspaces/{workspaceId}/teams/{teamId}/cycle-cadence",
				{
					params: { path: { workspaceId: workspace.id, teamId: team.id } },
					body: { lengthWeeks, startsOn },
				}
			);

			if (error) {
				failure = readCycleFailure(error);

				return;
			}

			if (data) saved = { kind: "enabled", cadence: data };

			await invalidate(keys.page(page.route.id));
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
			const { error } = await api.DELETE("/workspaces/{workspaceId}/teams/{teamId}/cycle-cadence", {
				params: { path: { workspaceId: workspace.id, teamId: team.id } },
			});

			if (error) {
				failure = readCycleFailure(error);

				return;
			}

			saved = { kind: "disabled" };

			await invalidate(keys.page(page.route.id));
		} catch {
			failure = { kind: "unavailable" };
		} finally {
			working = false;
		}
	}
</script>

<section class="flex flex-col gap-4">
	<div class="flex flex-col gap-1">
		<h2 class="text-md font-medium tracking-snug text-ink-900">Cycles</h2>
		<p class="text-sm leading-normal text-muted-foreground text-pretty">
			A repeating time box {team.name} plans against. Norn keeps the next three ready, so nobody has
			to remember to make them.
		</p>
	</div>

	{#if failure}
		<Alert.Root variant="destructive">
			<CircleX aria-hidden="true" />
			<Alert.Title>That did not go through</Alert.Title>
			<Alert.Description>{cycleFailureMessage(failure)}</Alert.Description>
		</Alert.Root>
	{/if}

	{#if current.kind === "loading"}
		<div class="h-24 animate-breathe rounded-lg bg-paper-2" aria-busy="true"></div>
	{:else if current.kind === "unavailable"}
		<Alert.Root variant="destructive">
			<CircleX aria-hidden="true" />
			<Alert.Title>We could not load the cadence</Alert.Title>
			<Alert.Description>Nothing changed. Wait a moment and try again.</Alert.Description>
		</Alert.Root>
	{:else if !enabled}
		<div class="flex flex-col gap-3 rounded-lg border border-line-subtle p-3">
			<p class="text-sm leading-normal text-muted-foreground text-pretty">
				{team.name} does not use cycles. Everything else about the team works exactly the same either
				way.
			</p>
			<div>
				<Button variant="secondary" disabled={disabled} onclick={() => save(length, weekday)}>
					{working ? "Starting" : "Use cycles for this team"}
				</Button>
			</div>
		</div>
	{:else if cadence}
		<div class="flex flex-col gap-4 rounded-lg border border-line-subtle p-3">
			<div class="flex flex-col gap-1.5">
				<span class="text-sm text-ink-900" id="cycle-length-label">Length</span>
				<Select.Root
					type="single"
					value={String(length)}
					onValueChange={(value) => save(Number(value), weekday)}
					disabled={disabled}
				>
					<Select.Trigger class="w-full" aria-labelledby="cycle-length-label">
						{lengthLabel(length)}
					</Select.Trigger>
					<Select.Content>
						{#each cycleLengths as weeks (weeks)}
							<Select.Item value={String(weeks)}>{lengthLabel(weeks)}</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
			</div>

			<div class="flex flex-col gap-1.5">
				<span class="text-sm text-ink-900" id="cycle-weekday-label">Starts on</span>
				<Select.Root
					type="single"
					value={String(weekday)}
					onValueChange={(value) => save(length, Number(value))}
					disabled={disabled}
				>
					<Select.Trigger class="w-full" aria-labelledby="cycle-weekday-label">
						{weekdayLabel(weekday)}
					</Select.Trigger>
					<Select.Content>
						{#each weekdays as day (day.value)}
							<Select.Item value={String(day.value)}>{day.label}</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
				<p class="text-sm leading-normal text-muted-foreground text-pretty">
					Changing either of these leaves cycles that have already started alone.
				</p>
			</div>

			{#if cadence.upcoming.length > 0}
				<div class="flex flex-col gap-1.5">
					<span class="text-sm text-ink-900">Queued</span>
					<ul class="flex flex-col gap-1">
						{#each cadence.upcoming as cycle (cycle.id)}
							<li class="flex items-baseline justify-between gap-2">
								<span class="text-sm text-ink-900">{cycle.name}</span>
								<span class="font-mono text-xs text-muted-foreground tabular-nums">
									{cycleWindow(cycle.startsOn, cycle.endsOn)}
								</span>
							</li>
						{/each}
					</ul>
				</div>
			{/if}

			<div class="flex flex-wrap gap-2">
				<Button variant="secondary" size="sm" href={teamCyclesPath(workspace.slug, team.key)}>
					All cycles
				</Button>
				<Button variant="ghost" size="sm" disabled={disabled} onclick={stop}>
					{working ? "Stopping" : "Stop using cycles"}
				</Button>
			</div>
		</div>
	{/if}
</section>

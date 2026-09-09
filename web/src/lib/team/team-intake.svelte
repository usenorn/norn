<script lang="ts">
	import { invalidate } from "$app/navigation";
	import { page } from "$app/state";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import Copy from "@lucide/svelte/icons/copy";
	import RefreshCw from "@lucide/svelte/icons/refresh-cw";
	import { api } from "$lib/api";
	import { keys } from "$lib/api/keys";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import {
		intakeFailureMessage,
		readIntakeFailure,
		type IntakeFailure,
		type IntakeSetting,
	} from "$lib/triage/intake";
	import type { Team } from "$lib/team/teams";

	let {
		workspace,
		team,
		setting,
		locked = false,
	}: {
		workspace: { id: string; slug: string };
		team: Team;
		setting: IntakeSetting;
		locked?: boolean;
	} = $props();

	// SvelteKit keeps this component across a move from one team's settings to another's, so
	// what was saved for the team just left would otherwise be shown, copied and rotated under
	// the name of the team now on screen. Everything local is tied to the team it belongs to.
	type Local = { at: string; setting?: IntakeSetting; failure?: IntakeFailure; copied?: boolean };

	let local = $state<Local | null>(null);
	let working = $state(false);

	const at = $derived(`${workspace.id}:${team.id}`);
	const mine = $derived<Local>(local?.at === at ? local : { at });
	const current = $derived<IntakeSetting>(mine.setting ?? setting);
	const failure = $derived(mine.failure ?? null);
	const copied = $derived(mine.copied ?? false);
	const address = $derived(current.kind === "on" ? current.address : null);
	const disabled = $derived(locked || working);
	const path = $derived({ workspaceId: workspace.id, teamId: team.id });

	function hold(held: Omit<Local, "at">, held_at: string) {
		// A reply that arrives after the move belongs to the team it was asked about, not the
		// one now on screen.
		if (held_at !== at) return;

		local = { at: held_at, ...held };
	}

	async function enable() {
		const asked = at;

		working = true;
		hold({}, asked);

		try {
			const { data, error } = await api.POST(
				"/workspaces/{workspaceId}/teams/{teamId}/intake-address",
				{ params: { path } }
			);

			if (error || !data) {
				hold({ failure: readIntakeFailure(error) }, asked);

				return;
			}

			hold({ setting: { kind: "on", address: data } }, asked);
			await invalidate(keys.page(page.route.id));
		} catch {
			hold({ failure: { kind: "unavailable" } }, asked);
		} finally {
			working = false;
		}
	}

	async function rotate() {
		const asked = at;

		working = true;
		hold({}, asked);

		try {
			const { data, error } = await api.POST(
				"/workspaces/{workspaceId}/teams/{teamId}/intake-address/rotation",
				{ params: { path } }
			);

			if (error || !data) {
				hold({ failure: readIntakeFailure(error) }, asked);

				return;
			}

			hold({ setting: { kind: "on", address: data } }, asked);
			await invalidate(keys.page(page.route.id));
		} catch {
			hold({ failure: { kind: "unavailable" } }, asked);
		} finally {
			working = false;
		}
	}

	async function stop() {
		const asked = at;

		working = true;
		hold({}, asked);

		try {
			const { error } = await api.DELETE(
				"/workspaces/{workspaceId}/teams/{teamId}/intake-address",
				{ params: { path } }
			);

			if (error) {
				hold({ failure: readIntakeFailure(error) }, asked);

				return;
			}

			hold({ setting: { kind: "off" } }, asked);
			await invalidate(keys.page(page.route.id));
		} catch {
			hold({ failure: { kind: "unavailable" } }, asked);
		} finally {
			working = false;
		}
	}

	async function copy() {
		if (!address) return;

		await navigator.clipboard.writeText(address.email);
		hold({ setting: mine.setting, copied: true }, at);
	}
</script>

<section class="flex flex-col gap-4">
	<div class="flex flex-col gap-1">
		<h2 class="text-md font-medium tracking-snug text-ink-900">Issues by email</h2>
		<p class="text-sm leading-normal text-muted-foreground text-pretty">
			Give {team.name} an address anyone can write to. Each message arrives as an issue, and it waits
			in triage when this team holds incoming work for review.
		</p>
	</div>

	{#if failure}
		<Alert.Root variant="destructive">
			<CircleX aria-hidden="true" />
			<Alert.Title>That did not stick</Alert.Title>
			<Alert.Description>{intakeFailureMessage(failure)}</Alert.Description>
		</Alert.Root>
	{/if}

	{#if current.kind === "loading"}
		<div class="h-24 animate-breathe rounded-lg bg-paper-2" aria-busy="true"></div>
	{:else if current.kind === "unavailable"}
		<p class="text-sm leading-normal text-muted-foreground">
			We could not read this team's address.
		</p>
	{:else if current.kind === "unconfigured"}
		<p class="text-sm leading-normal text-muted-foreground text-pretty">
			This instance takes no mail yet. An administrator sets the domain incoming mail arrives on.
		</p>
	{:else if !address}
		<div class="flex flex-col items-start gap-3 rounded-lg border border-line-default p-4">
			<p class="text-md leading-normal text-muted-foreground text-pretty">
				Nobody can write to {team.name} yet.
			</p>
			<Button variant="secondary" size="sm" {disabled} onclick={enable}>
				{working ? "Turning on" : "Take issues by email"}
			</Button>
		</div>
	{:else}
		<div class="flex flex-col gap-3 rounded-lg border border-line-default p-4">
			<div class="flex flex-wrap items-center gap-2">
				<code
					class="min-w-0 flex-1 truncate rounded-xs bg-paper-2 px-2 py-1.5 font-mono text-sm text-ink-900"
				>
					{address.email}
				</code>
				<Button variant="ghost" size="sm" onclick={copy}>
					<Copy aria-hidden="true" />
					{copied ? "Copied" : "Copy"}
				</Button>
				<Button variant="ghost" size="sm" {disabled} onclick={rotate}>
					<RefreshCw aria-hidden="true" />
					{working ? "Working" : "New address"}
				</Button>
			</div>
			<p class="text-sm leading-normal text-muted-foreground text-pretty">
				A new address retires this one at once, and mail sent to the old address is no longer filed.
			</p>
		</div>
		<div class="flex flex-wrap gap-2">
			<Button variant="ghost" size="sm" {disabled} onclick={stop}>
				{working ? "Working" : "Stop taking email"}
			</Button>
		</div>
	{/if}
</section>

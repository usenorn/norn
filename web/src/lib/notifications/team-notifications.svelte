<script lang="ts">
	import { invalidate } from "$app/navigation";
	import { page } from "$app/state";
	import { keys } from "$lib/api/keys";
	import Bell from "@lucide/svelte/icons/bell";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { api } from "$lib/api";
	import PreferenceGrid from "./preference-grid.svelte";
	import type { NotificationPreferences, TeamNotificationSetting } from "./notifications";
	import type { Team } from "$lib/team/teams";

	let {
		workspace,
		team,
		setting,
		locked = false,
	}: {
		workspace: { id: string; slug: string };
		team: Team;
		setting: TeamNotificationSetting;
		locked?: boolean;
	} = $props();

	let draft = $state.raw<NotificationPreferences | null>(null);
	let working = $state(false);
	let failed = $state(false);

	const ready = $derived(setting.kind === "ready" ? setting.settings : null);
	const preferences = $derived(draft ?? ready?.preferences ?? null);
	const overridden = $derived(ready?.overridden ?? false);
	const emailEnabled = $derived(ready?.emailEnabled ?? false);
	const dirty = $derived(
		draft !== null && JSON.stringify(draft) !== JSON.stringify(ready?.preferences)
	);
	const busy = $derived(locked || working);

	async function save() {
		if (!draft) return;

		await send(() =>
			api.PUT("/workspaces/{workspaceId}/teams/{teamId}/notification-settings", {
				params: { path: { workspaceId: workspace.id, teamId: team.id } },
				body: draft as NotificationPreferences,
			})
		);
	}

	async function clear() {
		await send(() =>
			api.DELETE("/workspaces/{workspaceId}/teams/{teamId}/notification-settings", {
				params: { path: { workspaceId: workspace.id, teamId: team.id } },
			})
		);
	}

	async function send(call: () => Promise<{ error?: unknown }>) {
		working = true;
		failed = false;

		try {
			const { error } = await call();

			if (error) {
				failed = true;

				return;
			}

			draft = null;
			await invalidate(keys.page(page.route.id));
		} catch {
			failed = true;
		} finally {
			working = false;
		}
	}
</script>

<section class="flex flex-col gap-4 rounded-lg border border-line-default p-4">
	<div class="flex flex-col gap-1">
		<h2 class="flex items-center gap-1.5 text-md font-medium tracking-snug text-ink-900">
			<Bell class="size-icon-row shrink-0 text-muted-foreground" aria-hidden="true" />
			Your notifications from {team.name}
		</h2>
		<p class="text-sm leading-normal text-muted-foreground text-pretty">
			{overridden
				? `These settings apply to ${team.key} alone. Drop them to go back to your workspace settings.`
				: `${team.key} currently follows your workspace settings. Change anything below to make it differ.`}
		</p>
	</div>

	{#if failed}
		<Alert.Root variant="destructive">
			<CircleX aria-hidden="true" />
			<Alert.Title>That did not save</Alert.Title>
			<Alert.Description>Nothing changed. Wait a moment and try again.</Alert.Description>
		</Alert.Root>
	{/if}

	{#if setting.kind === "loading"}
		<div class="h-40 animate-pulse rounded-lg bg-paper-2" aria-busy="true"></div>
	{:else if setting.kind === "unavailable"}
		<p class="text-sm leading-normal text-muted-foreground">
			We could not load these settings. Nothing changed &mdash; wait a moment and try again.
		</p>
	{:else if preferences}
		<PreferenceGrid
			{preferences}
			{emailEnabled}
			disabled={busy}
			idPrefix="team-{team.key}"
			onchange={(next) => (draft = next)}
		/>

		<div class="flex flex-wrap items-center gap-2">
			<Button size="sm" disabled={!dirty || busy} onclick={save}>
				{working ? "Saving…" : "Save for this team"}
			</Button>
			{#if overridden}
				<Button size="sm" variant="ghost" disabled={busy} onclick={clear}>
					Use my workspace settings
				</Button>
			{:else if dirty}
				<Button size="sm" variant="ghost" disabled={busy} onclick={() => (draft = null)}>
					Discard
				</Button>
			{/if}
		</div>
	{/if}
</section>

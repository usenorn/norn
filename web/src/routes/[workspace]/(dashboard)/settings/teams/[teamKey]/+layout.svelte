<script lang="ts">
	import { page } from "$app/state";
	import Archive from "@lucide/svelte/icons/archive";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import Users from "@lucide/svelte/icons/users";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { provideTeamSettings } from "$lib/team/team-settings-context";
	import { teamSettingsEntryAt } from "$lib/team/team-settings-sections";
	import { teamOf } from "$lib/team/team-settings";
	import { teamSettingsPath, type Team } from "$lib/team/teams";
	import { workspacePath } from "$lib/workspace/navigation";
	import { teamSettingsPreviewStates } from "./preview";
	import type { LayoutProps } from "./$types";

	let { data, children }: LayoutProps = $props();

	const preview = $derived(
		import.meta.env.DEV
			? teamSettingsPreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined
	);

	const slug = $derived(data.workspace.slug);
	const settings = $derived(preview?.settings ?? data.settings);
	const team = $derived(teamOf(settings));
	const archived = $derived(settings.kind === "archived");
	const readOnly = $derived(preview?.readOnly ?? data.readOnly);
	const entry = $derived(teamSettingsEntryAt(page.url.pathname));

	provideTeamSettings({
		get team() {
			return team as Team;
		},
		get archived() {
			return archived;
		},
		get readOnly() {
			return readOnly;
		},
	});
</script>

<svelte:head>
	<title>
		{entry ? `${entry.title} · ` : ""}{team ? team.name : "Team"} · {data.workspace.name} · Norn
	</title>
</svelte:head>

{#snippet crumb(href: string, label: string)}
	<a
		{href}
		class="text-md font-medium tracking-snug whitespace-nowrap text-muted-foreground motion-control hover:text-ink-900 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
	>
		{label}
	</a>
	<span class="text-md text-muted-foreground" aria-hidden="true">/</span>
{/snippet}

<div class="flex min-h-0 flex-1 flex-col">
	<div class="flex-none border-b border-line-default">
		<nav aria-label="Breadcrumb" class="flex h-11 min-w-0 items-center gap-2 pr-3 pl-4">
			<Users class="size-icon-toolbar shrink-0 text-muted-foreground" aria-hidden="true" />
			{#if team && entry}
				{@render crumb(workspacePath(slug, "/settings/teams"), "Teams")}
				{@render crumb(teamSettingsPath(slug, team.key), team.name)}
				<h1 class="min-w-0 truncate text-md font-medium tracking-snug text-ink-900">
					{entry.title}
				</h1>
			{:else if team}
				{@render crumb(workspacePath(slug, "/settings/teams"), "Teams")}
				<h1 class="min-w-0 truncate text-md font-medium tracking-snug text-ink-900">{team.name}</h1>
			{:else}
				<a
					href={workspacePath(slug, "/settings/teams")}
					class="text-md font-medium tracking-snug whitespace-nowrap text-muted-foreground motion-control hover:text-ink-900 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
				>
					Teams
				</a>
			{/if}
		</nav>
	</div>

	<div class="flex-1 overflow-auto">
		<div
			class="mx-auto flex w-full max-w-140 flex-col gap-6 px-4 py-6 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))]"
		>
			{#if settings.kind === "loading"}
				<div class="h-40 animate-breathe rounded-lg bg-paper-2" aria-busy="true"></div>
			{:else if settings.kind === "not_found"}
				<div class="flex flex-col gap-2">
					<h2 class="text-md font-medium tracking-snug text-ink-900">No team here</h2>
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						There is no team at this address in {data.workspace.name}, or it is private and you are
						not on it.
					</p>
					<div>
						<Button variant="secondary" size="sm" href={workspacePath(slug, "/settings/teams")}>
							Back to teams
						</Button>
					</div>
				</div>
			{:else if !team}
				<Alert.Root variant="destructive">
					<CircleX aria-hidden="true" />
					<Alert.Title>We could not load this team</Alert.Title>
					<Alert.Description>Nothing changed. Wait a moment and try again.</Alert.Description>
				</Alert.Root>
			{:else}
				{#if archived}
					<Alert.Root variant="destructive">
						<Archive aria-hidden="true" />
						<Alert.Title>This team is archived</Alert.Title>
						<Alert.Description>
							Its issues stay readable and {team.key}-1 style references still resolve. Nothing about
							the team can change until it is brought back from General.
						</Alert.Description>
					</Alert.Root>
				{/if}

				{#if readOnly}
					<Alert.Root variant="destructive">
						<CircleX aria-hidden="true" />
						<Alert.Title>You cannot change this team</Alert.Title>
						<Alert.Description>
							Only workspace administrators can change a team's settings. What you hear about from
							it under Notifications is still yours to set.
						</Alert.Description>
					</Alert.Root>
				{/if}

				{@render children()}
			{/if}
		</div>
	</div>
</div>

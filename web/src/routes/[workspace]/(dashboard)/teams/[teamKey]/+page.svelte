<script lang="ts">
	import { page } from "$app/state";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import Lock from "@lucide/svelte/icons/lock";
	import Settings from "@lucide/svelte/icons/settings";
	import Users from "@lucide/svelte/icons/users";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import TeamKey from "$lib/components/norn/team-key.svelte";
	import { teamIssuesPath } from "$lib/issues/listing";
	import { workspacePath } from "$lib/workspace/navigation";
	import { teamOverviewPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const slug = $derived(data.workspace.slug);
	const preview = $derived(
		import.meta.env.DEV
			? teamOverviewPreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined
	);
	const overview = $derived(preview?.overview ?? data.overview);
</script>

<svelte:head>
	<title>
		{overview.kind === "ready" ? overview.team.name : "Team"} · {data.workspace.name} · Norn
	</title>
</svelte:head>

<div class="flex min-h-0 flex-1 flex-col">
	<div class="flex-none border-b border-line-default">
		<div class="flex h-11 items-center gap-2 pr-3 pl-4">
			{#if overview.kind === "ready" && overview.team.visibility === "private"}
				<Lock class="size-icon-toolbar shrink-0 text-muted-foreground" aria-hidden="true" />
			{:else}
				<Users class="size-icon-toolbar shrink-0 text-muted-foreground" aria-hidden="true" />
			{/if}
			<h1 class="min-w-0 truncate text-md font-medium tracking-snug text-ink-900">
				{overview.kind === "ready" ? overview.team.name : "Team"}
			</h1>
			{#if overview.kind === "ready"}
				<TeamKey key={overview.team.key} />
				<div class="min-w-2 flex-1"></div>
				<Button
					variant="outline"
					size="sm"
					href={workspacePath(slug, `/settings/teams/${overview.team.key}`)}
				>
					<Settings aria-hidden="true" />
					Team settings
				</Button>
			{/if}
		</div>
	</div>

	<div class="relative flex-1 overflow-auto">
		<div
			class="mx-auto flex w-full max-w-140 flex-col gap-6 px-4 py-6 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))]"
		>
			{#if overview.kind === "loading"}
				<div class="h-24 animate-breathe rounded-lg bg-paper-2" aria-busy="true"></div>
			{:else if overview.kind === "not_found"}
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
			{:else if overview.kind === "unavailable"}
				<Alert.Root variant="destructive">
					<CircleX aria-hidden="true" />
					<Alert.Title>We could not load this team</Alert.Title>
					<Alert.Description>Nothing changed. Wait a moment and try again.</Alert.Description>
				</Alert.Root>
			{:else}
				<div class="flex flex-col gap-2">
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						An overview of what {overview.team.name} is working on will live here. For now, its issues
						and its settings are a click away.
					</p>
					<div class="flex flex-wrap gap-2">
						<Button
							variant="secondary"
							size="sm"
							href={teamIssuesPath(slug, overview.team.key)}
						>
							Issues
						</Button>
					</div>
				</div>
			{/if}
		</div>
	</div>
</div>

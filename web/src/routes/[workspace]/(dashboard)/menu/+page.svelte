<script lang="ts">
	import { page } from "$app/state";
	import Layers from "@lucide/svelte/icons/layers";
	import List from "@lucide/svelte/icons/list";
	import Lock from "@lucide/svelte/icons/lock";
	import Plus from "@lucide/svelte/icons/plus";
	import Settings from "@lucide/svelte/icons/settings";
	import Users from "@lucide/svelte/icons/users";
	import {
		Crosshair as CrosshairGlyph,
		List as ListGlyph,
		ListChecks as ListChecksGlyph,
		Target as TargetGlyph,
	} from "lucide";
	import AccountSwitcher from "$lib/account/account-switcher.svelte";
	import ConnectionIndicator from "$lib/realtime/connection-indicator.svelte";
	import { useRealtime } from "$lib/realtime/connection.svelte";
	import SidebarItem from "$lib/components/norn/sidebar-item.svelte";
	import SidebarSection from "$lib/components/norn/sidebar-section.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import { cyclePath } from "$lib/cycles/cycles";
	import { teamIssuesPath } from "$lib/issues/listing";
	import { teamProjectsPath } from "$lib/projects/projects";
	import { teamPath, teamSettingsPath } from "$lib/team/teams";
	import { viewEntries, viewsPath } from "$lib/views/views";
	import { isCurrent, isExactly, primaryNav, workspacePath } from "$lib/workspace/navigation";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const realtime = useRealtime();

	const slug = $derived(data.workspace.slug);
	const pathname = $derived(page.url.pathname);
	const search = $derived(page.url.searchParams);
	const current = $derived((href: string) => isCurrent(pathname, search, href));
	const exactly = $derived((href: string) => isExactly(pathname, href));

	const nav = $derived(primaryNav(slug, data.waiting, data.unread));
	const views = $derived(viewEntries(slug, data.views ?? []));
	const teams = $derived((data.teams ?? []).filter((team) => team.status === "active"));
	const cycleFor = $derived((teamId: string) => data.cycles.find((entry) => entry.teamId === teamId));
</script>

<svelte:head><title>Menu · {data.workspace.name} · Norn</title></svelte:head>

<div class="flex min-h-0 flex-1 flex-col overflow-y-auto">
	<nav
		aria-label="Menu"
		class="mx-auto flex w-full max-w-140 flex-col px-2 py-2 pb-[calc(--spacing(6)+env(safe-area-inset-bottom))]"
	>
		{#each nav as entry (entry.href)}
			<SidebarItem
				href={entry.href}
				label={entry.label}
				icon={entry.icon}
				iconClass={entry.iconClass}
				glyph={entry.glyph}
				glyphEngaged={entry.glyphEngaged}
				count={entry.count}
				active={current(entry.href)}
				size="touch"
			/>
		{/each}

		<SidebarSection label="Teams">
			{#snippet action()}
				<Button
					variant="ghost"
					size="icon-sm"
					href={workspacePath(slug, "/settings/teams?new")}
					aria-label="New team"
				>
					<Plus aria-hidden="true" />
				</Button>
			{/snippet}
		</SidebarSection>
		{#each teams as team (team.id)}
			{@const running = cycleFor(team.id)}
			<SidebarItem
				href={teamPath(slug, team.key)}
				label={team.name}
				icon={team.visibility === "private" ? Lock : Users}
				active={exactly(teamPath(slug, team.key))}
				size="touch"
			/>
			<SidebarItem
				href={teamIssuesPath(slug, team.key)}
				label="Issues"
				glyph={ListGlyph}
				glyphEngaged={ListChecksGlyph}
				indent
				active={current(teamIssuesPath(slug, team.key))}
				size="touch"
			/>
			<SidebarItem
				href={teamProjectsPath(slug, team.id)}
				label="Projects"
				glyph={TargetGlyph}
				glyphEngaged={CrosshairGlyph}
				indent
				active={current(teamProjectsPath(slug, team.id))}
				size="touch"
			/>
			{#if running}
				<SidebarItem
					href={cyclePath(slug, running.cycle)}
					label={running.cycle.name}
					icon={Layers}
					indent
					active={current(cyclePath(slug, running.cycle))}
					size="touch"
				/>
			{/if}
			<SidebarItem
				href={teamSettingsPath(slug, team.key)}
				label="Team settings"
				icon={Settings}
				indent
				active={current(teamSettingsPath(slug, team.key))}
				size="touch"
			/>
		{/each}
		{#if teams.length === 0}
			<SidebarItem
				href={workspacePath(slug, "/settings/teams?new")}
				label="Create a team"
				icon={Plus}
				indent
				active={false}
				size="touch"
			/>
		{/if}

		<SidebarSection label="Views">
			{#snippet action()}
				<Button
					variant="ghost"
					size="icon-sm"
					href={viewsPath(slug)}
					aria-label="Manage saved views"
				>
					<Settings aria-hidden="true" />
				</Button>
			{/snippet}
		</SidebarSection>
		{#each views as view (view.href)}
			<SidebarItem
				href={view.href}
				label={view.label}
				icon={view.icon}
				indent
				active={current(view.href)}
				size="touch"
			/>
		{/each}
		<SidebarItem
			href={viewsPath(slug)}
			label={views.length === 0 ? "Save a view" : "All views"}
			icon={views.length === 0 ? Plus : List}
			indent
			active={exactly(viewsPath(slug))}
			size="touch"
		/>

		<SidebarSection label="Workspace" />
		<SidebarItem
			href={workspacePath(slug, "/settings")}
			label="Settings"
			icon={Settings}
			active={current(workspacePath(slug, "/settings"))}
			size="touch"
		/>

		<SidebarSection label="Account" />
		<div class="flex h-11 items-center gap-2 px-1">
			<AccountSwitcher
				accounts={data.accounts}
				actingAccountId={data.member.id}
				workspace={{ slug, name: data.workspace.name }}
				trigger="person"
				class="min-w-0 flex-1"
			/>
		</div>
		{#if realtime}
			<div class="flex h-6 items-center px-2">
				<ConnectionIndicator state={realtime.state} />
			</div>
		{/if}
	</nav>
</div>

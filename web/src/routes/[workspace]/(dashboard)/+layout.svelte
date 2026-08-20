<script lang="ts">
	import { page } from "$app/state";
	import { goto, invalidate } from "$app/navigation";
	import { keys } from "$lib/api/keys";
	import Check from "@lucide/svelte/icons/check";
	import ChevronsUpDown from "@lucide/svelte/icons/chevrons-up-down";
	import Layers from "@lucide/svelte/icons/layers";
	import List from "@lucide/svelte/icons/list";
	import Plus from "@lucide/svelte/icons/plus";
	import Search from "@lucide/svelte/icons/search";
	import LogOut from "@lucide/svelte/icons/log-out";
	import Settings from "@lucide/svelte/icons/settings";
	import BadgeCheck from "@lucide/svelte/icons/badge-check";
	import Bell from "@lucide/svelte/icons/bell";
	import Bot from "@lucide/svelte/icons/bot";
	import AccountSwitcher from "$lib/account/account-switcher.svelte";
	import SearchPalette from "$lib/search/search-palette.svelte";
	import ConnectionIndicator from "$lib/realtime/connection-indicator.svelte";
	import StaleBanner from "$lib/realtime/stale-banner.svelte";
	import { invalidatedBy, provideRealtime } from "$lib/realtime/connection.svelte";
	import KeyRound from "@lucide/svelte/icons/key-round";
	import Network from "@lucide/svelte/icons/network";
	import Terminal from "@lucide/svelte/icons/terminal";
	import ProviderRequired from "$lib/workspace/provider-required.svelte";
	import Tags from "@lucide/svelte/icons/tags";
	import UserRound from "@lucide/svelte/icons/user-round";
	import Kbd from "$lib/components/norn/kbd.svelte";
	import NewIssueDialog from "$lib/issues/new-issue-dialog.svelte";
	import { provideNewIssue } from "$lib/issues/new-issue.svelte";
	import { showToast } from "$lib/toast/toasts";
	import type { CreationOutcome } from "$lib/issues/creating";
	import { calendarDate } from "$lib/time";
	import ShortcutHelp from "$lib/shortcuts/shortcut-help.svelte";
	import { bindShortcuts, holdShortcuts, provideShortcuts } from "$lib/shortcuts/registry.svelte";
	import { destinations } from "$lib/shortcuts/destinations";
	import { bindRoam, provideRoam } from "$lib/shortcuts/roam/roam.svelte";
	import {
		Crosshair as CrosshairGlyph,
		List as ListGlyph,
		ListChecks as ListChecksGlyph,
		Target as TargetGlyph,
	} from "lucide";
	import Lock from "@lucide/svelte/icons/lock";
	import Users from "@lucide/svelte/icons/users";
	import SidebarBranch from "$lib/components/norn/sidebar-branch.svelte";
	import SidebarItem from "$lib/components/norn/sidebar-item.svelte";
	import SidebarSection from "$lib/components/norn/sidebar-section.svelte";
	import WorkspaceMark from "$lib/components/norn/workspace-mark.svelte";
	import * as DropdownMenu from "$lib/components/ui/dropdown-menu/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import {
		isCurrent,
		isExactly,
		mobileNav,
		primaryNav,
		workspacePath,
	} from "$lib/workspace/navigation";
	import { viewEntries, viewsPath } from "$lib/views/views";
	import { cyclePath } from "$lib/cycles/cycles";
	import { teamIssuesPath } from "$lib/issues/listing";
	import { teamPath } from "$lib/team/teams";
	import { expansionCookie, toggledExpansion, writeExpanded } from "$lib/workspace/sidebar";
	import { projectPath, projectsPath } from "$lib/projects/projects";
	import type { LayoutProps } from "./$types";

	let { data, children }: LayoutProps = $props();

	const repairable = $derived(page.url.pathname.endsWith("/settings/authentication"));
	const narrowed = $derived(
		!repairable &&
			(import.meta.env.DEV && page.url.searchParams.get("state") === "provider_required"
				? true
				: data.narrowed)
	);


	const slug = $derived(data.workspace.slug);
	const pathname = $derived(page.url.pathname);
	const search = $derived(page.url.searchParams);
	const current = $derived((href: string) => isCurrent(pathname, search, href));
	const exactly = $derived((href: string) => isExactly(pathname, href));

	let searching = $state(false);

	let opened = $state.raw<{ source: string[]; keys: string[] } | null>(null);

	const expanded = $derived(
		opened && opened.source === data.expanded ? opened.keys : data.expanded
	);

	function toggle(key: string) {
		const keys = toggledExpansion(expanded, key);

		opened = { source: data.expanded, keys };

		const name = expansionCookie(data.member.id, data.workspace.id);
		const year = 60 * 60 * 24 * 365;

		document.cookie = `${name}=${writeExpanded(keys)}; path=/; max-age=${year}; samesite=lax`;
	}

	const realtime = provideRealtime();

	const workspaceId = $derived(data.workspace.id);
	const memberSlot = $derived(data.member.slot);

	$effect(() => {
		realtime.open(workspaceId, memberSlot);

		return () => realtime.close();
	});

	$effect(() => {
		const opened = workspaceId;

		return realtime.on((event) => realtime.refetch(...invalidatedBy(event, opened)));
	});

	$effect(() => {
		const wake = () => {
			if (document.visibilityState === "visible") void realtime.flush();
		};

		document.addEventListener("visibilitychange", wake);

		return () => document.removeEventListener("visibilitychange", wake);
	});


	const shortcuts = provideShortcuts();

	let helping = $state(false);

	bindRoam(provideRoam());

	const raising = provideNewIssue();

	holdShortcuts(() => raising.open);

	bindShortcuts({
		search: () => (searching = true),
		help: () => (helping = true),
	});

	$effect(() => {
		if (teams.length === 0) return;

		return shortcuts.register("issue-new", () => raising.raise());
	});

	async function settled(outcome: CreationOutcome) {
		const handled = raising.onsettled;

		if (handled) {
			await handled(outcome);

			return;
		}

		if (outcome.kind === "refused") {
			showToast(outcome.failure);

			if (outcome.input) raising.raise(outcome.input);

			return;
		}

		showToast(`Created ${outcome.issue.reference}`, {
			href: workspacePath(slug, `/issues/${outcome.issue.reference}`),
		});

		await invalidate(keys.issues(data.workspace.id));
	}

	$effect(() => {
		const released = destinations(slug).map((destination) =>
			shortcuts.register(destination.id, () => void goto(destination.href))
		);

		return () => released.forEach((release) => release());
	});

	const nav = $derived(primaryNav(slug, data.waiting, data.unread));
	const views = $derived(viewEntries(slug, data.views ?? []));
	const tabs = $derived(mobileNav(slug));

	const teams = $derived((data.teams ?? []).filter((team) => team.status === "active"));
	const today = $derived(calendarDate(data.now, data.workspace.timezone));
	const cycleFor = $derived((teamId: string) => data.cycles.find((entry) => entry.teamId === teamId));
</script>

<div class="flex h-dvh bg-background">
	<aside
		class="hidden w-sidebar flex-none flex-col overflow-y-auto border-r border-line-default bg-card px-2 py-2.5 md:flex"
	>
		<AccountSwitcher
			accounts={data.accounts}
			actingAccountId={data.member.id}
			workspace={{ slug, name: data.workspace.name }}
		/>

		<button
			type="button"
			onclick={() => (searching = true)}
			class="keycap mt-2.5 mb-1 flex h-control-md w-full items-center gap-2 rounded-sm border border-line-default bg-card px-2 text-md text-muted-foreground motion-control [--keycap-lip:var(--paper-2)] hover:bg-accent hover:text-ink-900 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
		>
			<Search class="size-icon-toolbar shrink-0" aria-hidden="true" />
			<span class="flex-1 text-left">Search</span>
			<Kbd keys="⌘ K" />
		</button>

		<nav aria-label="Workspace" data-roam="nav">
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
				/>
			{/each}

			<SidebarSection label="Teams">
				{#snippet action()}
					<Button
						variant="ghost"
						size="icon-xs"
						href={workspacePath(slug, "/settings/teams?new")}
						aria-label="New team"
					>
						<Plus aria-hidden="true" />
					</Button>
				{/snippet}
			</SidebarSection>
			{#each teams as team (team.id)}
				{@const running = cycleFor(team.id)}
				<SidebarBranch
					id="team-{team.key}"
					href={teamPath(slug, team.key)}
					label={team.name}
					icon={team.visibility === "private" ? Lock : Users}
					active={exactly(teamPath(slug, team.key))}
					expanded={expanded.includes(team.key)}
					ontoggle={() => toggle(team.key)}
				>
					<SidebarItem
						href={teamIssuesPath(slug, team.key)}
						label="Issues"
						glyph={ListGlyph}
						glyphEngaged={ListChecksGlyph}
						indent
						active={current(teamIssuesPath(slug, team.key))}
					/>
					{#if running}
						<SidebarItem
							href={cyclePath(slug, running.cycle)}
							label={running.cycle.name}
							icon={Layers}
							indent
							active={current(cyclePath(slug, running.cycle))}
						/>
					{/if}
				</SidebarBranch>
			{/each}
			{#if teams.length === 0}
				<SidebarItem
					href={workspacePath(slug, "/settings/teams?new")}
					label="Create a team"
					icon={Plus}
					indent
					active={false}
				/>
			{/if}

			<SidebarSection label="Projects">
				{#snippet action()}
					<Button
						variant="ghost"
						size="icon-xs"
						href={workspacePath(slug, "/projects?new")}
						aria-label="New project"
					>
						<Plus aria-hidden="true" />
					</Button>
				{/snippet}
			</SidebarSection>
			{#each data.projects as project (project.id)}
				<SidebarItem
					href={projectPath(slug, project)}
					label={project.name}
					glyph={TargetGlyph}
					glyphEngaged={CrosshairGlyph}
					indent
					active={current(projectPath(slug, project))}
				/>
			{/each}
			<SidebarItem
				href={projectsPath(slug)}
				label={data.projects.length === 0 ? "Start a project" : "All projects"}
				icon={data.projects.length === 0 ? Plus : List}
				indent
				active={exactly(projectsPath(slug))}
			/>

			<SidebarSection label="Views">
				{#snippet action()}
					<Button
						variant="ghost"
						size="icon-xs"
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
				/>
			{/each}
			<SidebarItem
				href={viewsPath(slug)}
				label={views.length === 0 ? "Save a view" : "All views"}
				icon={views.length === 0 ? Plus : List}
				indent
				active={exactly(viewsPath(slug))}
			/>
		</nav>

		<div class="flex-1"></div>

		<div class="flex h-6 items-center px-2">
			<ConnectionIndicator state={realtime.state} />
		</div>

		<div class="mt-2 flex h-9.5 items-center gap-2 border-t border-line-default px-1.5">
			<AccountSwitcher
				accounts={data.accounts}
				actingAccountId={data.member.id}
				workspace={{ slug, name: data.workspace.name }}
				trigger="person"
			/>
			<Button
				href={workspacePath(slug, "/settings")}
				variant="ghost"
				size="icon-sm"
				aria-label="Settings"
			>
				<Settings aria-hidden="true" />
			</Button>
		</div>
	</aside>

	<div class="flex min-w-0 flex-1 flex-col bg-card">
		<header
			class="flex h-13 flex-none items-center gap-2.5 border-b border-line-default px-4 md:hidden"
		>
			<WorkspaceMark name={data.workspace.name} class="size-6" />
			<span class="min-w-0 flex-1 truncate text-lg font-medium tracking-snug text-ink-900">
				{data.workspace.name}
			</span>
			<Button variant="ghost" size="icon" aria-label="Search" onclick={() => (searching = true)}>
				<Search class="size-icon-toolbar" aria-hidden="true" />
			</Button>
			<AccountSwitcher
				accounts={data.accounts}
				actingAccountId={data.member.id}
				workspace={{ slug, name: data.workspace.name }}
				trigger="avatar"
			/>
		</header>

		<main class="flex min-h-0 flex-1 flex-col">
			<StaleBanner shown={realtime.degraded} off={realtime.state === "off"} />
			{#if narrowed}
				<ProviderRequired workspace={data.workspace} />
			{:else}
				{@render children()}
			{/if}
		</main>

		<nav
			aria-label="Sections"
			class="flex h-15 flex-none items-stretch border-t border-line-default bg-card pb-[env(safe-area-inset-bottom)] md:hidden"
		>
			{#each tabs as entry, index (entry.href)}
				{@const Glyph = entry.icon}
				{#if index === 2}
					<div class="flex flex-1 items-center justify-center">
						<Button size="touch" aria-label="New task" onclick={() => raising.raise()}>
							<Plus aria-hidden="true" />
						</Button>
					</div>
				{/if}
				<a
					href={entry.href}
					data-active={current(entry.href)}
					aria-current={current(entry.href) ? "page" : undefined}
					class="relative flex flex-1 flex-col items-center justify-center gap-0.5 text-2xs font-medium text-muted-foreground data-[active=true]:text-ink-900 data-[active=true]:before:absolute data-[active=true]:before:-top-px data-[active=true]:before:right-4 data-[active=true]:before:left-4 data-[active=true]:before:h-0.5 data-[active=true]:before:bg-primary"
				>
					{#if Glyph}
						<Glyph class="size-5" aria-hidden="true" />
					{/if}
					{entry.label}
				</a>
			{/each}
		</nav>
	</div>
</div>

<svelte:window onkeydown={(event) => shortcuts.handle(event)} />

<ShortcutHelp bind:open={helping} />

{#if teams.length > 0}
	<NewIssueDialog
		bind:open={raising.open}
		workspaceId={data.workspace.id}
		{teams}
		states={{}}
		members={data.members}
		labels={data.labels}
		projects={data.projects}
		{today}
		now={data.now}
		prefill={raising.prefill}
		onraising={raising.onraising}
		onsettled={settled}
	/>
{/if}

<SearchPalette
	bind:open={searching}
	workspaceId={data.workspace.id}
	workspaceSlug={slug}
/>

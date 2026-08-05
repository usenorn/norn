<script lang="ts">
	import { page } from "$app/state";
	import Check from "@lucide/svelte/icons/check";
	import ChevronsUpDown from "@lucide/svelte/icons/chevrons-up-down";
	import Layers from "@lucide/svelte/icons/layers";
	import List from "@lucide/svelte/icons/list";
	import Target from "@lucide/svelte/icons/target";
	import Lock from "@lucide/svelte/icons/lock";
	import Plus from "@lucide/svelte/icons/plus";
	import Search from "@lucide/svelte/icons/search";
	import LogOut from "@lucide/svelte/icons/log-out";
	import Settings from "@lucide/svelte/icons/settings";
	import Bell from "@lucide/svelte/icons/bell";
	import SearchPalette from "$lib/search/search-palette.svelte";
	import ConnectionIndicator from "$lib/realtime/connection-indicator.svelte";
	import StaleBanner from "$lib/realtime/stale-banner.svelte";
	import { provideRealtime } from "$lib/realtime/connection.svelte";
	import KeyRound from "@lucide/svelte/icons/key-round";
	import Terminal from "@lucide/svelte/icons/terminal";
	import ProviderRequired from "$lib/workspace/provider-required.svelte";
	import Tags from "@lucide/svelte/icons/tags";
	import UserRound from "@lucide/svelte/icons/user-round";
	import Users from "@lucide/svelte/icons/users";
	import Kbd from "$lib/components/norn/kbd.svelte";
	import SidebarItem from "$lib/components/norn/sidebar-item.svelte";
	import SidebarSection from "$lib/components/norn/sidebar-section.svelte";
	import WorkspaceMark from "$lib/components/norn/workspace-mark.svelte";
	import * as Avatar from "$lib/components/ui/avatar/index.js";
	import * as DropdownMenu from "$lib/components/ui/dropdown-menu/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import {
		isCurrent,
		mobileNav,
		primaryNav,
		workspacePath,
	} from "$lib/workspace/navigation";
	import { viewEntries, viewsPath } from "$lib/views/views";
	import { cyclePath } from "$lib/cycles/cycles";
	import { projectPath, projectsPath } from "$lib/projects/projects";
	import { initialsOf } from "$lib/team/members";
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

	let searching = $state(false);

	const realtime = provideRealtime();

	$effect(() => {
		realtime.open(data.workspace.id);

		return () => realtime.close();
	});

	$effect(() => {
		const stop = realtime.on((event) => {
			// The open issue and the badges patch themselves from the payload; everything that
			// depends on a server-evaluated filter asks for a coalesced refetch instead.
			if (event.kind === "notification.arrived" || event.kind.startsWith("issue.")) {
				realtime.refetch();
			}
		});

		return stop;
	});


	function openSearch(event: KeyboardEvent) {
		if (event.key !== "k" || !(event.metaKey || event.ctrlKey)) return;

		const target = event.target as HTMLElement | null;

		if (
			target instanceof HTMLInputElement ||
			target instanceof HTMLTextAreaElement ||
			target?.isContentEditable
		) {
			return;
		}

		event.preventDefault();
		searching = true;
	}

	const nav = $derived(primaryNav(slug, data.waiting, data.unread));
	const views = $derived(viewEntries(slug, data.views ?? []));
	const tabs = $derived(mobileNav(slug));

	const initials = $derived(initialsOf(data.member.name));
	const teams = $derived((data.teams ?? []).filter((team) => team.status === "active"));
	const cycleFor = $derived((teamId: string) => data.cycles.find((entry) => entry.teamId === teamId));
</script>

<div class="flex h-dvh bg-background">
	<aside
		class="hidden w-sidebar flex-none flex-col overflow-y-auto border-r border-line-default bg-card px-2 py-2.5 md:flex"
	>
		<DropdownMenu.Root>
			<DropdownMenu.Trigger
				class="flex h-8.5 w-full items-center gap-2 rounded-md px-1.5 text-left transition-colors duration-110 ease-out hover:bg-accent focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
				aria-label="Switch workspace"
			>
				<WorkspaceMark name={data.workspace.name} />
				<span class="min-w-0 flex-1 truncate text-md font-medium tracking-snug text-ink-900">
					{data.workspace.name}
				</span>
				<ChevronsUpDown class="size-icon-row shrink-0 text-muted-foreground" aria-hidden="true" />
			</DropdownMenu.Trigger>
			<DropdownMenu.Content align="start" sideOffset={4}>
				<DropdownMenu.Label>Workspaces</DropdownMenu.Label>
				{#each data.workspaces as option (option.slug)}
					<DropdownMenu.Item>
						{#snippet child({ props })}
							<a href={workspacePath(option.slug, "/my-tasks")} {...props}>
								<WorkspaceMark name={option.name} class="size-4.5 rounded-xs text-2xs" />
								<span class="min-w-0 flex-1 truncate">{option.name}</span>
								{#if option.slug === slug}
									<Check class="text-ink-600" aria-hidden="true" />
								{/if}
							</a>
						{/snippet}
					</DropdownMenu.Item>
				{/each}
				<DropdownMenu.Separator />
				<DropdownMenu.Item>
					{#snippet child({ props })}
						<a href={workspacePath(slug, "/settings")} {...props}>
							<Settings aria-hidden="true" />
							Workspace settings
						</a>
					{/snippet}
				</DropdownMenu.Item>
				<DropdownMenu.Item>
					{#snippet child({ props })}
						<a href={workspacePath(slug, "/settings/teams")} {...props}>
							<Users aria-hidden="true" />
							Teams
						</a>
					{/snippet}
				</DropdownMenu.Item>
				<DropdownMenu.Item>
					{#snippet child({ props })}
						<a href={workspacePath(slug, "/settings/members")} {...props}>
							<UserRound aria-hidden="true" />
							Members
						</a>
					{/snippet}
				</DropdownMenu.Item>
				<DropdownMenu.Item>
					{#snippet child({ props })}
						<a href={workspacePath(slug, "/settings/labels")} {...props}>
							<Tags aria-hidden="true" />
							Labels
						</a>
					{/snippet}
				</DropdownMenu.Item>
				<DropdownMenu.Item>
					{#snippet child({ props })}
						<a href={workspacePath(slug, "/settings/notifications")} {...props}>
							<Bell aria-hidden="true" />
							Notifications
						</a>
					{/snippet}
				</DropdownMenu.Item>
				<DropdownMenu.Item>
					{#snippet child({ props })}
						<a href={workspacePath(slug, "/settings/authentication")} {...props}>
							<KeyRound aria-hidden="true" />
							Authentication
						</a>
					{/snippet}
				</DropdownMenu.Item>
				<DropdownMenu.Separator />
				<DropdownMenu.Item>
					{#snippet child({ props })}
						<a href="/settings/tokens" {...props}>
							<Terminal aria-hidden="true" />
							Your API tokens
						</a>
					{/snippet}
				</DropdownMenu.Item>
				<DropdownMenu.Separator />
				<DropdownMenu.Item>
					{#snippet child({ props })}
						<a href="/create-workspace" {...props}>
							<Plus aria-hidden="true" />
							Create a workspace
						</a>
					{/snippet}
				</DropdownMenu.Item>
				<DropdownMenu.Item variant="destructive">
					{#snippet child({ props })}
						<a href="/sign-in" {...props}>
							<LogOut aria-hidden="true" />
							Sign out
						</a>
					{/snippet}
				</DropdownMenu.Item>
			</DropdownMenu.Content>
		</DropdownMenu.Root>

		<button
			type="button"
			onclick={() => (searching = true)}
			class="keycap mt-2.5 mb-1 flex h-control-md w-full items-center gap-2 rounded-sm border border-line-default bg-card px-2 text-md text-muted-foreground transition-colors duration-110 ease-out [--keycap-lip:var(--paper-2)] hover:bg-accent hover:text-ink-900 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
		>
			<Search class="size-icon-toolbar shrink-0" aria-hidden="true" />
			<span class="flex-1 text-left">Search</span>
			<Kbd keys="⌘ K" />
		</button>

		<nav aria-label="Workspace">
			{#each nav as entry (entry.href)}
				<SidebarItem
					href={entry.href}
					label={entry.label}
					icon={entry.icon}
					iconClass={entry.iconClass}
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
				<SidebarItem
					href={workspacePath(slug, `/settings/teams/${team.key}`)}
					label={team.name}
					icon={team.visibility === "private" ? Lock : Users}
					indent
					active={current(workspacePath(slug, `/settings/teams/${team.key}`))}
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
					icon={Target}
					indent
					active={current(projectPath(slug, project))}
				/>
			{/each}
			<SidebarItem
				href={projectsPath(slug)}
				label={data.projects.length === 0 ? "Start a project" : "All projects"}
				icon={data.projects.length === 0 ? Plus : List}
				indent
				active={current(projectsPath(slug))}
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
				active={current(viewsPath(slug))}
			/>
		</nav>

		<div class="flex-1"></div>

		<div class="flex h-6 items-center px-2">
			<ConnectionIndicator state={realtime.state} />
		</div>

		<div class="mt-2 flex h-9.5 items-center gap-2 border-t border-line-default px-1.5">
			<Avatar.Root size="sm">
				<Avatar.Fallback>{initials}</Avatar.Fallback>
			</Avatar.Root>
			<span class="min-w-0 flex-1 truncate text-sm text-ink-600">{data.member.name}</span>
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
			<Avatar.Root size="sm">
				<Avatar.Fallback>{initials}</Avatar.Fallback>
			</Avatar.Root>
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
						<Button size="touch" aria-label="New task">
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

<svelte:window onkeydown={openSearch} />

<SearchPalette
	bind:open={searching}
	workspaceId={data.workspace.id}
	workspaceSlug={slug}
/>

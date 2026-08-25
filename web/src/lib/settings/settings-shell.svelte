<script lang="ts">
	import { page } from "$app/state";
	import ArrowLeft from "@lucide/svelte/icons/arrow-left";
	import Menu from "@lucide/svelte/icons/menu";
	import type { Snippet } from "svelte";
	import SidebarSection from "$lib/components/norn/sidebar-section.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Separator } from "$lib/components/ui/separator/index.js";
	import * as Sheet from "$lib/components/ui/sheet/index.js";
	import { settingsEntryFor, type SettingsNavigationSection } from "$lib/settings/navigation";
	import SettingsNavigation from "$lib/settings/settings-navigation.svelte";
	import { provideShortcuts } from "$lib/shortcuts/registry.svelte";
	import { bindRoam, provideRoam } from "$lib/shortcuts/roam/roam.svelte";

	let {
		kindLabel,
		scopeName,
		backHref,
		backLabel,
		sections,
		scope,
		children,
	}: {
		kindLabel: string;
		scopeName: string;
		backHref: string;
		backLabel: string;
		sections: SettingsNavigationSection[];
		scope: Snippet;
		children: Snippet;
	} = $props();

	let navigationOpen = $state(false);
	const shortcuts = provideShortcuts();
	bindRoam(provideRoam());

	const pathname = $derived(page.url.pathname);
	const current = $derived(settingsEntryFor(pathname, sections));
</script>

<div class="flex h-dvh overflow-hidden bg-background">
	<aside
		class="hidden w-sidebar flex-none flex-col overflow-y-auto border-r border-line-default bg-card px-2 py-2.5 md:flex"
	>
		<Button variant="ghost" size="sm" href={backHref} class="w-full justify-start">
			<ArrowLeft data-icon="inline-start" aria-hidden="true" />
			<span class="min-w-0 truncate">{backLabel}</span>
		</Button>

		<SidebarSection label={kindLabel} />
		{@render scope()}

		<SettingsNavigation {sections} {pathname} />
	</aside>

	<div class="flex min-w-0 flex-1 flex-col bg-card">
		<header
			class="flex h-13 flex-none items-center gap-2 border-b border-line-default px-3 md:hidden"
		>
			<Sheet.Root bind:open={navigationOpen}>
				<Sheet.Trigger>
					{#snippet child({ props })}
						<Button {...props} variant="ghost" size="icon" aria-label="Open settings navigation">
							<Menu aria-hidden="true" />
						</Button>
					{/snippet}
				</Sheet.Trigger>
				<Sheet.Content side="left" class="w-72 max-w-[calc(100vw-2rem)]">
					<Sheet.Header class="pr-12">
						<Sheet.Title>{kindLabel}</Sheet.Title>
						<Sheet.Description>{scopeName}</Sheet.Description>
					</Sheet.Header>
					<Separator />
					<div class="flex min-h-0 flex-1 flex-col overflow-y-auto px-2 pb-[env(safe-area-inset-bottom)]">
						<Button variant="ghost" size="sm" href={backHref} class="mt-2 w-full justify-start">
							<ArrowLeft data-icon="inline-start" aria-hidden="true" />
							<span class="min-w-0 truncate">{backLabel}</span>
						</Button>

						<SidebarSection label="Scope" />
						{@render scope()}

						<SettingsNavigation
							{sections}
							{pathname}
							onnavigate={() => (navigationOpen = false)}
						/>
					</div>
				</Sheet.Content>
			</Sheet.Root>

			<div class="flex min-w-0 flex-1 flex-col">
				<span class="truncate text-md font-medium tracking-snug text-ink-900">{scopeName}</span>
				<span class="truncate font-mono text-2xs tracking-caps text-muted-foreground uppercase">
					{current?.label ?? kindLabel}
				</span>
			</div>

			<Button variant="ghost" size="icon" href={backHref} aria-label={backLabel}>
				<ArrowLeft aria-hidden="true" />
			</Button>
		</header>

		<main class="flex min-h-0 flex-1 flex-col">
			{@render children()}
		</main>
	</div>
</div>

<svelte:window onkeydown={(event) => shortcuts.handle(event)} />

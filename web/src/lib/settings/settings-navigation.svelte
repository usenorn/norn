<script lang="ts">
	import SidebarItem from "$lib/components/norn/sidebar-item.svelte";
	import SidebarSection from "$lib/components/norn/sidebar-section.svelte";
	import { settingsEntryFor, type SettingsNavigationSection } from "$lib/settings/navigation";

	let {
		sections,
		pathname,
		onnavigate,
	}: {
		sections: SettingsNavigationSection[];
		pathname: string;
		onnavigate?: () => void;
	} = $props();

	const current = $derived(settingsEntryFor(pathname, sections));
</script>

<nav aria-label="Settings">
	{#each sections as section (section.label)}
		<SidebarSection label={section.label} />
		{#each section.entries as entry (entry.href)}
			<SidebarItem
				href={entry.href}
				label={entry.label}
				icon={entry.icon}
				active={current?.href === entry.href}
				onclick={onnavigate}
			/>
		{/each}
	{/each}
</nav>

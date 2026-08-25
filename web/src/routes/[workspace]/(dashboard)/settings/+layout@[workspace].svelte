<script lang="ts">
	import AccountSwitcher from "$lib/account/account-switcher.svelte";
	import { withSlot } from "$lib/account/accounts";
	import { workspaceSettingsNavigation } from "$lib/settings/navigation";
	import SettingsShell from "$lib/settings/settings-shell.svelte";
	import { workspacePath } from "$lib/workspace/navigation";
	import type { LayoutProps } from "./$types";

	let { data, children }: LayoutProps = $props();

	const slug = $derived(data.workspace.slug);
	const sections = $derived(workspaceSettingsNavigation(slug));
	const back = $derived(withSlot(workspacePath(slug, "/my-tasks"), data.member.slot));
</script>

<SettingsShell
	kindLabel="Workspace settings"
	scopeName={data.workspace.name}
	backHref={back}
	backLabel={`Back to ${data.workspace.name}`}
	{sections}
>
	{#snippet scope()}
		<AccountSwitcher
			accounts={data.accounts}
			actingAccountId={data.member.id}
			workspace={{ slug, name: data.workspace.name }}
			context="workspace-settings"
		/>
	{/snippet}

	{@render children()}
</SettingsShell>

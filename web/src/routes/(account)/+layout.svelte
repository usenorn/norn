<script lang="ts">
	import AccountSwitcher from "#lib/account/account-switcher.svelte";
	import { withSlot } from "#lib/account/accounts.js";
	import { userSettingsNavigation } from "#lib/settings/navigation.js";
	import SettingsShell from "#lib/settings/settings-shell.svelte";
	import { workspacePath } from "#lib/workspace/navigation.js";
	import type { LayoutProps } from "./$types";

	let { data, children }: LayoutProps = $props();

	const back = $derived(
		data.workspaces.length > 0
			? withSlot(workspacePath(data.workspaces[0].slug, "/my-tasks"), data.account.slot)
			: "/"
	);
	const backLabel = $derived(`Back to ${data.workspaces[0]?.name ?? "Norn"}`);
</script>

<SettingsShell
	kindLabel="User settings"
	scopeName={data.account.name}
	backHref={back}
	{backLabel}
	sections={userSettingsNavigation}
>
	{#snippet scope()}
		<AccountSwitcher
			accounts={data.accounts}
			actingAccountId={data.account.id}
			trigger="person"
			context="user-settings"
		/>
	{/snippet}

	{@render children()}
</SettingsShell>

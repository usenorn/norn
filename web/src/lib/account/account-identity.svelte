<script lang="ts">
	import * as Avatar from "$lib/components/ui/avatar/index.js";
	import { initialsOf, type Account } from "$lib/account/accounts";
	import type { AvatarSize } from "$lib/components/ui/avatar/avatar.svelte";

	let {
		account,
		size = "sm",
		class: className,
	}: { account: Account; size?: AvatarSize; class?: string } = $props();
</script>

<div class={["flex min-w-0 items-center gap-2", className]}>
	<Avatar.Root {size}>
		{#if account.avatarUrl}
			<Avatar.Image src={account.avatarUrl} alt="" />
		{/if}
		<Avatar.Fallback>{initialsOf(account.displayName)}</Avatar.Fallback>
	</Avatar.Root>
	<div class="flex min-w-0 flex-col">
		<span class="truncate text-sm font-medium text-ink-900">{account.displayName}</span>
		<span class="truncate text-xs text-muted-foreground">{account.email}</span>
	</div>
</div>

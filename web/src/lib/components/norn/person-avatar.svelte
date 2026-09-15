<script lang="ts">
	import * as Avatar from "$lib/components/ui/avatar/index.js";
	import type { AvatarSize } from "$lib/components/ui/avatar/avatar.svelte";
	import { initialsOf } from "$lib/account/accounts";
	import { avatarToneOf } from "$lib/account/avatar-tone";

	let {
		accountId,
		name,
		avatarUrl,
		size = "default",
		title,
		class: className,
	}: {
		accountId: string;
		name: string;
		avatarUrl?: string;
		size?: AvatarSize;
		title?: string;
		class?: string;
	} = $props();

	const tone = $derived(avatarToneOf(accountId));
</script>

<Avatar.Root {size} {tone} {title} data-tone={tone} class={className}>
	{#if avatarUrl}
		<Avatar.Image src={avatarUrl} alt="" />
	{/if}
	<Avatar.Fallback>{initialsOf(name)}</Avatar.Fallback>
</Avatar.Root>

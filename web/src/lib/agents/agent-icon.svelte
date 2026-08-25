<script lang="ts">
	import Bot from "@lucide/svelte/icons/bot";
	import GitPullRequest from "@lucide/svelte/icons/git-pull-request";
	import Inbox from "@lucide/svelte/icons/inbox";
	import Pencil from "@lucide/svelte/icons/pencil";
	import ScrollText from "@lucide/svelte/icons/scroll-text";
	import Search from "@lucide/svelte/icons/search";
	import ShieldCheck from "@lucide/svelte/icons/shield-check";
	import Sparkles from "@lucide/svelte/icons/sparkles";
	import Target from "@lucide/svelte/icons/target";
	import Terminal from "@lucide/svelte/icons/terminal";
	import type { AgentIcon } from "./agents";

	let { icon, class: className }: { icon: AgentIcon; class?: string } = $props();
</script>

{#if icon === "inbox"}
	<Inbox class={className} aria-hidden="true" />
{:else if icon === "search"}
	<Search class={className} aria-hidden="true" />
{:else if icon === "terminal"}
	<Terminal class={className} aria-hidden="true" />
{:else if icon === "pencil"}
	<Pencil class={className} aria-hidden="true" />
{:else if icon === "git-pull-request"}
	<GitPullRequest class={className} aria-hidden="true" />
{:else if icon === "shield-check"}
	<ShieldCheck class={className} aria-hidden="true" />
{:else if icon === "scroll-text"}
	<ScrollText class={className} aria-hidden="true" />
{:else if icon === "target"}
	<Target class={className} aria-hidden="true" />
{:else if icon === "sparkles"}
	<Sparkles class={className} aria-hidden="true" />
{:else}
	<Bot class={className} aria-hidden="true" />
{/if}

<script lang="ts">
	import Check from "@lucide/svelte/icons/check";
	import MessageSquare from "@lucide/svelte/icons/message-square";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import type { ListedRepository } from "./review";

	let {
		repositories,
		onpick,
	}: { repositories: ListedRepository[]; onpick?: () => void } = $props();

	const letters = { added: "A", deleted: "D", renamed: "R", modified: "M" };
	const tones = {
		added: "text-success",
		deleted: "text-destructive",
		renamed: "text-ink-600",
		modified: "text-muted-foreground",
	};

	function split(path: string): { dir: string; name: string } {
		const cut = path.lastIndexOf("/");

		return cut === -1 ? { dir: "", name: path } : { dir: path.slice(0, cut + 1), name: path.slice(cut + 1) };
	}
</script>

<nav aria-label="Changed files" class="flex min-w-0 flex-col gap-4">
	{#each repositories as held (held.repository)}
		<div class="flex min-w-0 flex-col gap-1">
			<Eyebrow>{held.repository}</Eyebrow>
			{#if held.note}
				<p class="text-2xs text-muted-foreground">{held.note}</p>
			{/if}
			<ul class="flex min-w-0 flex-col">
				{#each held.files as listed (listed.id)}
					{@const named = split(listed.file.path)}
					<li class="min-w-0">
						<a
							href={`#${listed.id}`}
							onclick={() => onpick?.()}
							class="flex min-w-0 items-center gap-2 rounded-xs px-1.5 py-1 text-xs hover:bg-accent focus-visible:outline-2 focus-visible:outline-ring"
							title={listed.file.path}
						>
							<span class="w-3 shrink-0 font-mono text-2xs {tones[listed.file.status]}" aria-hidden="true">
								{letters[listed.file.status]}
							</span>
							<span class="min-w-0 flex-1 truncate">
								<span class="text-muted-foreground">{named.dir}</span><span
									class={listed.viewed ? "text-muted-foreground" : "text-ink-900"}>{named.name}</span
								>
							</span>
							{#if listed.threads > 0}
								<span class="inline-flex items-center gap-0.5 font-mono text-2xs text-muted-foreground">
									<MessageSquare class="size-3" aria-hidden="true" />
									{listed.threads}
									<span class="sr-only">threads</span>
								</span>
							{/if}
							{#if listed.viewed}
								<Check class="size-3 shrink-0 text-success" aria-label="Viewed" />
							{/if}
						</a>
					</li>
				{/each}
			</ul>
		</div>
	{/each}
</nav>

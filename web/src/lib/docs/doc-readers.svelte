<script lang="ts">
	import PersonAvatar from "$lib/components/norn/person-avatar.svelte";
	import type { DocReaders } from "./docs";

	let { readers, line }: { readers: DocReaders; line: string } = $props();
</script>

<div class="flex min-w-0 flex-col gap-2">
	<p class="text-sm text-ink-900">{line}</p>
	{#if readers.kind === "author"}
		<div class="flex items-center gap-2">
			<PersonAvatar accountId={readers.accountId ?? "author"} name={readers.name} size="sm" />
			<span class="truncate text-sm text-ink-600">{readers.name}</span>
		</div>
	{:else if readers.kind === "people"}
		{#if readers.people.length === 0}
			<p class="text-xs text-muted-foreground">Nobody is in this audience yet.</p>
		{:else}
			<ul class="flex flex-wrap items-center gap-1.5" aria-label="People who can read it">
				{#each readers.people as person (person.accountId)}
					<li class="flex min-w-0 items-center gap-1.5 rounded-xs border border-line-subtle py-0.5 pr-2 pl-0.5">
						<PersonAvatar accountId={person.accountId} name={person.name} size="xs" />
						<span class="max-w-32 truncate text-xs text-ink-600">{person.name}</span>
					</li>
				{/each}
				{#if readers.more}
					<li class="text-xs text-muted-foreground">and others</li>
				{/if}
			</ul>
		{/if}
	{:else}
		<p class="text-xs text-muted-foreground">We could not list who is in this audience right now.</p>
	{/if}
</div>

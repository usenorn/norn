<script lang="ts">
	import { goto } from "$app/navigation";
	import CircleDot from "@lucide/svelte/icons/circle-dot";
	import MessageSquare from "@lucide/svelte/icons/message-square";
	import Layers from "@lucide/svelte/icons/layers";
	import UserRound from "@lucide/svelte/icons/user-round";
	import Users from "@lucide/svelte/icons/users";
	import * as Command from "$lib/components/ui/command/index.js";
	import { api } from "$lib/api";
	import { destinations } from "$lib/shortcuts/destinations";
	import { holdShortcuts } from "$lib/shortcuts/registry.svelte";
	import { displayKeys, isApplePlatform, shortcutOf } from "$lib/shortcuts/shortcuts";
	import {
		kindLabels,
		listingFor,
		resultPath,
		searchDebounceMs,
		searchPath,
		type SearchKind,
		type SearchListing,
	} from "./search";

	let {
		open = $bindable(false),
		workspaceId,
		workspaceSlug,
	}: {
		open?: boolean;
		workspaceId: string;
		workspaceSlug: string;
	} = $props();

	const apple = isApplePlatform();

	holdShortcuts(() => open);

	let typed = $state("");
	let listing = $state.raw<SearchListing>({ kind: "idle" });
	let timer: ReturnType<typeof setTimeout> | undefined;
	let inflight = 0;

	const glyphs: Record<SearchKind, typeof CircleDot> = {
		issue: CircleDot,
		comment: MessageSquare,
		project: Layers,
		team: Users,
		person: UserRound,
	};

	function type(value: string) {
		typed = value;
		clearTimeout(timer);

		if (value.trim() === "") {
			listing = { kind: "idle" };

			return;
		}

		listing = { kind: "searching" };
		timer = setTimeout(() => void run(value), searchDebounceMs);
	}

	async function run(value: string) {
		const sequence = ++inflight;

		try {
			const { data, error } = await api.GET("/workspaces/{workspaceId}/search", {
				params: { path: { workspaceId }, query: { q: value } },
			});

			if (sequence !== inflight) return;

			listing = error ? { kind: "unavailable" } : listingFor(data);
		} catch {
			if (sequence === inflight) listing = { kind: "unavailable" };
		}
	}

	function go(href: string) {
		open = false;
		typed = "";
		listing = { kind: "idle" };
		void goto(href);
	}

	function seeEverything() {
		if (typed.trim() === "") return;

		go(searchPath(workspaceSlug, typed));
	}

	$effect(() => {
		if (!open) {
			clearTimeout(timer);
			typed = "";
			listing = { kind: "idle" };
		}
	});
</script>

<Command.Dialog bind:open shouldFilter={false}>
	<Command.Input
		placeholder="Search issues, comments, projects, teams and people…"
		value={typed}
		oninput={(event) => type(event.currentTarget.value)}
	/>
	<Command.List>
		{#if listing.kind === "idle"}
			<Command.Empty>Type to search this workspace.</Command.Empty>
			<Command.Group heading="Go to">
				{#each destinations(workspaceSlug) as destination (destination.id)}
					<Command.Item value={destination.id} onSelect={() => go(destination.href)}>
						<span class="flex-1">{destination.label}</span>
						<Command.Shortcut>
							{displayKeys(shortcutOf(destination.id).keys[0], apple)}
						</Command.Shortcut>
					</Command.Item>
				{/each}
			</Command.Group>
		{:else if listing.kind === "searching"}
			<Command.Loading>Searching…</Command.Loading>
		{:else if listing.kind === "unavailable"}
			<Command.Empty>Search is unavailable. Nothing changed — try again in a moment.</Command.Empty>
		{:else if listing.kind === "no_matches"}
			<Command.Empty>
				Nothing matches “{typed}”{listing.fuzzy ? ", even approximately" : ""}.
			</Command.Empty>
		{:else}
			{#if listing.fuzzy}
				<div class="px-3 py-2 text-sm text-muted-foreground">
					No exact match. Showing the closest titles instead.
				</div>
			{/if}
			{#each listing.groups as group (group.kind)}
				{@const Glyph = glyphs[group.kind]}
				<Command.Group heading={kindLabels[group.kind]}>
					{#each group.results as result (result.id)}
						<Command.Item
							value={result.id}
							onSelect={() => go(resultPath(workspaceSlug, result))}
						>
							<Glyph class="size-icon-row shrink-0 text-muted-foreground" aria-hidden="true" />
							{#if result.reference}
								<span class="shrink-0 font-mono text-2xs text-muted-foreground">
									{result.reference}
								</span>
							{/if}
							<span class="min-w-0 flex-1 truncate">{result.title}</span>
							{#if result.excerpt}
								<span class="min-w-0 max-w-60 truncate text-sm text-muted-foreground">
									{result.excerpt}
								</span>
							{/if}
						</Command.Item>
					{/each}
				</Command.Group>
			{/each}
			<Command.Separator />
			<Command.Group>
				<Command.Item value="see-all" onSelect={seeEverything}>
					<span class="flex-1">See all results for “{typed}”</span>
					<Command.Shortcut>↵</Command.Shortcut>
				</Command.Item>
			</Command.Group>
		{/if}
	</Command.List>
</Command.Dialog>

<script lang="ts">
	import { goto } from "$app/navigation";
	import { page } from "$app/state";
	import CircleDot from "@lucide/svelte/icons/circle-dot";
	import Layers from "@lucide/svelte/icons/layers";
	import MessageSquare from "@lucide/svelte/icons/message-square";
	import Search from "@lucide/svelte/icons/search";
	import UserRound from "@lucide/svelte/icons/user-round";
	import Users from "@lucide/svelte/icons/users";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import {
		kindLabels,
		resultPath,
		searchDebounceMs,
		searchPath,
		type SearchKind,
		type SearchListing,
	} from "$lib/search/search";
	import { onDateAndTime } from "$lib/time";
	import { workspacePath } from "$lib/workspace/navigation";
	import { searchPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV ? searchPreviewStates[page.url.searchParams.get("state") ?? ""] : undefined
	);

	const slug = $derived(data.workspace.slug);
	const listing = $derived<SearchListing>(preview?.listing ?? data.listing);
	const query = $derived(preview?.query ?? data.query);

	let typed = $state("");
	let timer: ReturnType<typeof setTimeout> | undefined;

	$effect(() => {
		typed = query;
	});

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
		timer = setTimeout(commit, searchDebounceMs);
	}

	function commit() {
		clearTimeout(timer);

		void goto(searchPath(slug, typed), {
			replaceState: true,
			keepFocus: true,
			noScroll: true,
		});
	}
</script>

<svelte:head><title>{query ? `${query} · Search` : "Search"} · {data.workspace.name} · Norn</title></svelte:head>

<div class="flex min-h-0 flex-1 flex-col">
	<div class="flex-none border-b border-line-default">
		<div class="flex h-11 items-center gap-2 pr-3 pl-4">
			<Search class="size-icon-toolbar shrink-0 text-muted-foreground" aria-hidden="true" />
			<h1 class="text-md font-medium tracking-snug whitespace-nowrap text-ink-900">Search</h1>
		</div>
	</div>

	<div class="flex-1 overflow-auto">
		<div
			class="mx-auto flex w-full max-w-160 flex-col gap-4 px-4 py-5 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))]"
		>
			<Input
				type="search"
				value={typed}
				placeholder="Search issues, comments, projects, teams and people…"
				aria-label="Search this workspace"
				oninput={(event) => type(event.currentTarget.value)}
				onkeydown={(event) => event.key === "Enter" && commit()}
			/>

			{#if listing.kind === "idle"}
				<p class="text-md leading-normal text-muted-foreground text-pretty">
					Type a word, part of a word, or an issue reference like
					<span class="font-mono text-sm">ENG-412</span>.
				</p>
			{:else if listing.kind === "searching"}
				<div class="h-40 animate-breathe rounded-lg bg-paper-2" aria-busy="true"></div>
			{:else if listing.kind === "unavailable"}
				<p class="text-md leading-normal text-muted-foreground">
					We could not run that search. Nothing changed &mdash; wait a moment and try again.
				</p>
			{:else if listing.kind === "no_matches"}
				<div class="flex flex-col items-center gap-3 px-6 py-16 text-center">
					<p class="text-md font-medium tracking-snug text-ink-900">
						Nothing matches &ldquo;{query}&rdquo;
					</p>
					<p class="max-w-90 text-md leading-normal text-muted-foreground text-pretty">
						{listing.fuzzy
							? "Not even approximately. Try fewer words, or a different one."
							: "Try fewer words, or part of one."}
					</p>
					<Button href={workspacePath(slug, "/issues")} variant="secondary" size="sm">
						Browse issues instead
					</Button>
				</div>
			{:else}
				{#if listing.fuzzy}
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						Nothing matched &ldquo;{query}&rdquo; exactly. These are the closest titles.
					</p>
				{/if}

				{#each listing.groups as group (group.kind)}
					{@const Glyph = glyphs[group.kind]}
					<section class="flex flex-col" aria-label={kindLabels[group.kind]}>
						<div class="flex h-7.5 items-center gap-2 border-b border-line-default">
							<span
								class="font-mono text-2xs font-medium tracking-eyebrow text-ink-600 uppercase"
							>
								{kindLabels[group.kind]}
							</span>
							<span class="h-px flex-1 bg-line-default" aria-hidden="true"></span>
							{#if group.more}
								<span class="text-sm text-muted-foreground">more available</span>
							{/if}
						</div>

						<ul>
							{#each group.results as result (result.id)}
								<li class="border-b border-line-subtle">
									<a
										href={resultPath(slug, result)}
										class="flex min-w-0 items-center gap-2 py-2 focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-ring"
									>
										<Glyph
											class="size-icon-row shrink-0 text-muted-foreground"
											aria-hidden="true"
										/>
										<span class="min-w-0 flex-1">
											<span class="flex min-w-0 items-center gap-2">
												{#if result.reference}
													<span class="shrink-0 font-mono text-2xs text-muted-foreground">
														{result.reference}
													</span>
												{/if}
												<span class="min-w-0 truncate text-md tracking-snug text-ink-900">
													{result.title}
												</span>
											</span>
											{#if result.excerpt}
												<span class="block min-w-0 truncate text-sm text-muted-foreground">
													{result.excerpt}
												</span>
											{/if}
										</span>
										<time
											class="shrink-0 text-sm whitespace-nowrap text-muted-foreground"
											datetime={result.updatedAt}
										>
											{onDateAndTime(result.updatedAt, data.workspace.timezone)}
										</time>
									</a>
								</li>
							{/each}
						</ul>
					</section>
				{/each}
			{/if}
		</div>
	</div>
</div>

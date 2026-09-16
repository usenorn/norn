<script lang="ts">
	import { untrack } from "svelte";
	import { goto, invalidate } from "$app/navigation";
	import { page } from "$app/state";
	import ChevronLeft from "@lucide/svelte/icons/chevron-left";
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import CircleDot from "@lucide/svelte/icons/circle-dot";
	import CommandIcon from "@lucide/svelte/icons/command";
	import Info from "@lucide/svelte/icons/info";
	import Layers from "@lucide/svelte/icons/layers";
	import MessageSquare from "@lucide/svelte/icons/message-square";
	import Plus from "@lucide/svelte/icons/plus";
	import Search from "@lucide/svelte/icons/search";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import UserRound from "@lucide/svelte/icons/user-round";
	import Users from "@lucide/svelte/icons/users";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import * as Command from "$lib/components/ui/command/index.js";
	import * as Dialog from "$lib/components/ui/dialog/index.js";
	import { Progress } from "$lib/components/ui/progress/index.js";
	import Kbd from "$lib/components/norn/kbd.svelte";
	import PersonAvatar from "$lib/components/norn/person-avatar.svelte";
	import StatusIcon from "$lib/components/norn/status-icon.svelte";
	import Tag from "$lib/components/norn/tag.svelte";
	import { api } from "$lib/api";
	import { keys } from "$lib/api/keys";
	import { copyText } from "$lib/clipboard";
	import { applyChange } from "$lib/command/apply";
	import { paletteCommands, stepLabel } from "$lib/command/commands";
	import {
		paletteDestinations,
		peopleDestinations,
		resultDestinationId,
	} from "$lib/command/destinations";
	import { paletteListing, type SearchState } from "$lib/command/listing";
	import {
		createLabel,
		enterHint,
		entryId,
		footNote,
		groupsOf,
		modeOf,
		placeholderFor,
		scopedIssues,
		scopeSubject,
		sharedTeam,
		type CommandRun,
		type CommandStep,
		type Destination,
		type PaletteCommand,
		type PaletteEntry,
		type RecentEntry,
		type StepOption,
	} from "$lib/command/model";
	import { readRecents, referenceOf, rememberRecent, resolveRecents } from "$lib/command/recents";
	import { useCommandTargets } from "$lib/command/scope.svelte";
	import {
		appliedLine,
		changeFor,
		failedTitle,
		queuedLine,
		stepOptions,
	} from "$lib/command/steps";
	import type { Cycle, TeamCycle } from "$lib/cycles/cycles";
	import { useNewIssue } from "$lib/issues/new-issue.svelte";
	import type { Label, LabelColor } from "$lib/labels/labels";
	import { toggleDensity } from "$lib/layout/density";
	import type { Project } from "$lib/projects/projects";
	import { chordOf, holdShortcuts } from "$lib/shortcuts/registry.svelte";
	import { isApplePlatform, shortcutOf } from "$lib/shortcuts/shortcuts";
	import type { WorkflowState } from "$lib/team/states";
	import type { Team } from "$lib/team/teams";
	import { showToast } from "$lib/toast/toasts";
	import type { SavedView } from "$lib/views/views";
	import type { Membership } from "$lib/workspace/members";
	import { workspacePath } from "$lib/workspace/navigation";
	import { palettePreviewStates } from "./preview";
	import {
		searchDebounceMs,
		searchPath,
		resultPath,
		type SearchKind,
	} from "./search";

	let {
		open = $bindable(false),
		workspaceId,
		workspaceSlug,
		teams,
		cycles,
		views,
		projects,
		members,
		labels,
		states,
		creatable,
	}: {
		open?: boolean;
		workspaceId: string;
		workspaceSlug: string;
		teams: Team[];
		cycles: TeamCycle[];
		views: SavedView[];
		projects: Project[];
		members: Membership[];
		labels: Label[];
		states: WorkflowState[];
		creatable: boolean;
	} = $props();

	const apple = isApplePlatform();
	const targets = useCommandTargets();
	const raising = useNewIssue();

	holdShortcuts(() => open);

	const preview = $derived(
		import.meta.env.DEV
			? palettePreviewStates[page.url.searchParams.get("palette") ?? ""]
			: undefined
	);

	$effect(() => {
		if (preview) open = true;
	});

	let typed = $state("");
	let step = $state.raw<CommandStep | null>(null);
	let search = $state.raw<SearchState>({ kind: "idle" });
	let run = $state.raw<CommandRun>({ kind: "idle" });
	let recents = $state.raw<RecentEntry[]>([]);
	let recentIssues = $state.raw(new Map<string, Destination>());
	let teamCycles = $state.raw<{ teamId: string; cycles: Cycle[] } | null>(null);
	let highlighted = $state("");
	let asked: StepOption | null = null;
	let timer: ReturnType<typeof setTimeout> | undefined;
	let inflight = 0;

	const glyphs: Record<SearchKind, typeof CircleDot> = {
		issue: CircleDot,
		comment: MessageSquare,
		project: Layers,
		team: Users,
		person: UserRound,
	};

	const dots: Record<LabelColor, string> = {
		neutral: "bg-label-neutral",
		cyan: "bg-label-cyan",
		blue: "bg-label-blue",
		violet: "bg-label-violet",
		orchid: "bg-label-orchid",
		magenta: "bg-label-magenta",
	};

	const scope = $derived(preview?.scope ?? targets.scope);
	const shownTyped = $derived(preview?.typed ?? typed);
	const shownRun = $derived(preview?.run ?? run);
	const shownStep = $derived(preview ? preview.step : step);
	const mode = $derived(modeOf(shownTyped, shownStep));
	const subject = $derived(scopeSubject(scope));
	const team = $derived(sharedTeam(scope));
	const running = $derived(shownRun.kind === "running");

	const destinations = $derived(
		paletteDestinations({ workspace: workspaceSlug, teams, cycles, views, projects, apple })
	);
	const people = $derived(peopleDestinations(workspaceSlug, members));
	const commands = $derived(paletteCommands(scope, apple));

	const options = $derived(
		shownStep
			? stepOptions(shownStep.kind, {
					teamId: team,
					members,
					states,
					labels,
					cycles:
						teamCycles && teamCycles.teamId === team
							? teamCycles.cycles
							: cycles.map((entry) => entry.cycle),
				})
			: []
	);

	const listing = $derived(
		preview?.listing ??
			paletteListing({
				mode,
				search,
				recents: resolveRecents(recents, [...destinations, ...people], recentIssues),
				destinations,
				commands,
				options,
				everything:
					mode.query === ""
						? null
						: {
								id: "search-all",
								label: `See all results for “${mode.query}”`,
								href: searchPath(workspaceSlug, mode.query),
								icon: Search,
							},
			})
	);

	const groups = $derived(groupsOf(listing));
	const searching = $derived(
		running || listing.kind === "searching" || listing.kind === "slow"
	);

	$effect(() => {
		const opened = open;
		const workspace = workspaceId;

		untrack(() => (opened ? reopen(workspace) : reset()));
	});

	function reopen(workspace: string) {
		recents = readRecents(workspace);
		void resolveIssueRecents(recents);
	}

	function reset() {
		clearTimeout(timer);
		typed = "";
		step = null;
		search = { kind: "idle" };
		run = { kind: "idle" };
		asked = null;
	}

	async function resolveIssueRecents(entries: RecentEntry[]) {
		const wanted = entries.filter(
			(entry) => entry.id.startsWith("issue:") && !recentIssues.has(entry.id)
		);

		const found = await Promise.all(
			wanted.map(async (entry): Promise<Destination | null> => {
				const reference = referenceOf(entry.href);

				if (!reference) return null;

				const read = await api
					.GET("/workspaces/{workspaceId}/issues/by-reference/{reference}", {
						params: { path: { workspaceId, reference } },
					})
					.catch(() => undefined);

				if (!read?.data) return null;

				return {
					id: entry.id,
					label: read.data.title,
					href: workspacePath(workspaceSlug, `/issues/${read.data.reference}`),
					context: read.data.reference,
					icon: CircleDot,
				};
			})
		);

		const next = new Map(recentIssues);

		for (const destination of found) {
			if (destination) next.set(destination.id, destination);
		}

		recentIssues = next;
	}

	function type(value: string) {
		typed = value;
		run = { kind: "idle" };
		clearTimeout(timer);

		const next = modeOf(value, step);

		if (next.kind !== "search" || next.query === "") {
			search = { kind: "idle" };

			return;
		}

		search = { kind: "searching" };
		timer = setTimeout(() => void find(next.query), searchDebounceMs);
	}

	async function find(query: string) {
		const sequence = ++inflight;

		try {
			const { data, error } = await api.GET("/workspaces/{workspaceId}/search", {
				params: { path: { workspaceId }, query: { q: query } },
			});

			if (sequence !== inflight) return;

			search = error || !data
				? { kind: "unavailable" }
				: {
						kind: "ready",
						groups: data.groups.filter((group) => group.results.length > 0),
						fuzzy: data.fuzzy,
					};
		} catch {
			if (sequence === inflight) search = { kind: "unavailable" };
		}
	}

	function remember(id: string, href: string) {
		recents = rememberRecent(workspaceId, { id, href });
		open = false;
	}

	function popStep() {
		step = null;
		typed = "";
		run = { kind: "idle" };
		asked = null;
	}

	async function command(entry: PaletteCommand) {
		if (entry.step) {
			step = { kind: entry.step, label: stepLabel(entry) };
			typed = "";
			search = { kind: "idle" };

			if (entry.step === "cycle" && team) void loadCycles(team);

			return;
		}

		open = false;

		switch (entry.id) {
			case "issue-new":
				raising.raise();
				break;
			case "copy-link":
				await copyText(
					scopedIssues(scope)
						.map((issue) => `${page.url.origin}${workspacePath(workspaceSlug, `/issues/${issue.reference}`)}`)
						.join("\n"),
					`Copied link to ${subject}`
				);
				break;
			case "open-triage":
				await goto(workspacePath(workspaceSlug, "/triage"));
				break;
			case "toggle-density":
				toggleDensity();
				break;
		}
	}

	async function loadCycles(teamId: string) {
		const read = await api
			.GET("/workspaces/{workspaceId}/cycles", {
				params: { path: { workspaceId }, query: { teamId } },
			})
			.catch(() => undefined);

		if (read?.data) teamCycles = { teamId, cycles: read.data };
	}

	async function apply(option: StepOption) {
		if (!step || running) return;

		const kind = step.kind;
		const issues = scopedIssues(scope);
		const surface = targets.surface;

		asked = option;
		run = { kind: "running", entry: option.value };

		const outcome = await applyChange(
			workspaceId,
			issues.map((issue) => issue.id),
			changeFor(kind, option.value)
		);

		if (outcome.kind === "refused") {
			run = { kind: "failed", title: failedTitle(kind, subject), detail: outcome.detail };

			return;
		}

		showToast(outcome.kind === "applied" ? appliedLine(kind, subject, option.label) : queuedLine(subject));
		open = false;

		await Promise.all([
			surface?.onapplied?.(),
			invalidate(keys.issues(workspaceId)),
			...issues.map((issue) => invalidate(keys.issue(issue.id))),
		]);
	}

	function create() {
		if (!creatable || listing.kind !== "no_matches") return;

		open = false;
		raising.raise({ title: listing.query });
	}

	function select(entry: PaletteEntry) {
		if (entry.kind === "command") void command(entry.command);
		if (entry.kind === "option") void apply(entry.option);
	}

	function keydown(event: KeyboardEvent) {
		const chord = chordOf(event);

		if (shortcutOf("search-find").keys.includes(chord)) event.preventDefault();

		if (chord === "mod+enter") {
			event.preventDefault();
			create();
		}

		if (event.key === "Backspace" && typed === "" && step) {
			event.preventDefault();
			popStep();
		}
	}
</script>

{#snippet row(entry: PaletteEntry)}
	{#if entry.kind === "destination"}
		{@const Glyph = entry.destination.icon}
		<span class="inline-flex w-3.75 flex-none justify-center text-muted-foreground">
			{#if Glyph}<Glyph aria-hidden="true" />{/if}
		</span>
		<span class="min-w-0 flex-1 truncate">{entry.destination.label}</span>
		{#if entry.destination.context}
			<span class="max-w-47.5 truncate text-sm text-muted-foreground">{entry.destination.context}</span>
		{/if}
		{#if entry.destination.keys}
			<Kbd keys={entry.destination.keys} />
		{/if}
	{:else if entry.kind === "result"}
		{@const Glyph = glyphs[entry.result.kind]}
		<span class="inline-flex w-3.75 flex-none justify-center text-muted-foreground">
			<Glyph aria-hidden="true" />
		</span>
		<span class="min-w-0 flex-1 truncate">{entry.result.title}</span>
		{#if entry.result.excerpt}
			<span class="max-w-47.5 truncate text-sm text-muted-foreground">{entry.result.excerpt}</span>
		{/if}
		{#if entry.result.reference}
			<span class="font-mono text-xs text-muted-foreground">{entry.result.reference}</span>
		{/if}
	{:else if entry.kind === "command"}
		{@const Glyph = entry.command.icon}
		<span class="inline-flex w-3.75 flex-none justify-center text-muted-foreground">
			{#if Glyph}<Glyph aria-hidden="true" />{/if}
		</span>
		<span class="min-w-0 flex-1 truncate">{entry.command.label}</span>
		{#if entry.command.keys}
			<Kbd keys={entry.command.keys} />
		{/if}
	{:else}
		{@const option = entry.option}
		<span class="inline-flex w-3.75 flex-none justify-center text-muted-foreground">
			{#if option.person}
				<PersonAvatar accountId={option.person.accountId} name={option.person.name} size="xs" />
			{:else if option.category}
				<StatusIcon category={option.category} name={option.label} decorative />
			{:else if option.dot}
				<span class="size-2 rounded-[2px] {dots[option.dot]}" aria-hidden="true"></span>
			{:else}
				<Layers aria-hidden="true" />
			{/if}
		</span>
		<span class="min-w-0 flex-1 truncate">{option.label}</span>
		{#if option.hint}
			<span class="font-mono text-xs text-muted-foreground">{option.hint}</span>
		{/if}
		{#if shownRun.kind === "running" && shownRun.entry === option.value}
			<span class="font-mono text-2xs tracking-eyebrow text-muted-foreground uppercase">Running</span>
		{/if}
	{/if}
{/snippet}

<Dialog.Root bind:open>
	<Dialog.Content
		variant="palette"
		showCloseButton={false}
		onEscapeKeydown={(event) => {
			if (!shownStep) return;

			event.preventDefault();
			popStep();
		}}
	>
		<Dialog.Title class="sr-only">Search or run a command</Dialog.Title>
		<Dialog.Description class="sr-only">{placeholderFor(listing)}</Dialog.Description>
		<Command.Root
			shouldFilter={false}
			loop
			bind:value={highlighted}
			class="bg-transparent data-[running=true]:opacity-72"
			data-running={running}
		>
			<Command.Input
				variant="palette"
				placeholder={placeholderFor(listing)}
				value={shownTyped}
				oninput={(event) => type(event.currentTarget.value)}
				onkeydown={keydown}
			>
				{#snippet leading()}
					{#if mode.kind === "search"}
						<Search class="size-3.75 shrink-0 text-muted-foreground" aria-hidden="true" />
					{:else}
						<CommandIcon class="size-3.75 shrink-0 text-ink-600" aria-hidden="true" />
					{/if}
					{#if shownStep}
						<Button variant="chip" size="chip" onclick={popStep}>
							<ChevronLeft aria-hidden="true" />{shownStep.label}
						</Button>
					{/if}
				{/snippet}
				{#if mode.kind === "commands"}
					<Tag name="Command" />
				{/if}
				<Kbd keys="Esc" />
			</Command.Input>

			{#if searching}
				<Progress indeterminate class="h-0.5 rounded-none" aria-label="Searching" />
			{/if}

			{#if shownRun.kind === "failed"}
				<div class="px-3 pt-2.5 pb-0.5">
					<Alert.Root variant="destructive">
						<CircleAlert aria-hidden="true" />
						<Alert.Title>{shownRun.title}</Alert.Title>
						<Alert.Description>{shownRun.detail}</Alert.Description>
						<Alert.Action placement="below">
							<Button variant="secondary" size="sm" onclick={() => asked && apply(asked)}>Retry</Button>
							<Button variant="ghost" size="sm" onclick={popStep}>Cancel</Button>
						</Alert.Action>
					</Alert.Root>
				</div>
			{:else if listing.kind === "unavailable"}
				<div class="px-3 pt-2.5 pb-0.5">
					<Alert.Root variant="warning">
						<TriangleAlert aria-hidden="true" />
						<Alert.Title>Search is unavailable</Alert.Title>
						<Alert.Description>
							The search service isn’t responding. Recent items and commands still work.
						</Alert.Description>
					</Alert.Root>
				</div>
			{:else if listing.kind === "slow"}
				<div class="px-3 pt-2.5 pb-0.5">
					<Alert.Root variant="muted">
						<Info aria-hidden="true" />
						<Alert.Title>Large workspace</Alert.Title>
						<Alert.Description>
							{listing.scanning.toLocaleString("en-US")} issues · server results land in a moment, local matches are already listed.
						</Alert.Description>
					</Alert.Root>
				</div>
			{/if}

			<Command.List variant="palette">
				{#if listing.kind === "no_matches"}
					<div class="flex flex-col items-center gap-3 px-2.5 pt-5 pb-3">
						<span class="text-md text-muted-foreground">
							Nothing matches “{listing.query}” in this workspace.
						</span>
						{#if creatable}
							<Button variant="outline" size="sm" onclick={create}>
								<Plus aria-hidden="true" />{createLabel(listing.query)}
								<Kbd keys={apple ? "⌘ ↵" : "Ctrl ↵"} />
							</Button>
						{/if}
					</div>
				{:else if listing.kind === "step" && groups[0]?.entries.length === 0}
					<div class="px-2.5 py-5 text-center text-md text-muted-foreground">
						Nothing matches “{mode.query}”.
					</div>
				{:else if listing.kind === "results" && listing.fuzzy}
					<div class="px-2 py-1.5 text-sm text-muted-foreground">
						No exact match. Showing the closest titles instead.
					</div>
				{/if}
				{#each groups as group (group.id)}
					<Command.Group heading={group.heading || undefined} meta={group.meta}>
						{#each group.entries as entry (entryId(entry))}
							{#if entry.kind === "destination" || entry.kind === "result"}
								{@const href =
									entry.kind === "destination"
										? entry.destination.href
										: resultPath(workspaceSlug, entry.result)}
								<Command.LinkItem
									variant="palette"
									value="{group.id}:{entryId(entry)}"
									{href}
									disabled={running}
									onSelect={() =>
										remember(
											entry.kind === "destination"
												? entry.destination.id
												: resultDestinationId(entry.result),
											href
										)}
								>
									{@render row(entry)}
								</Command.LinkItem>
							{:else}
								<Command.Item
									variant="palette"
									value="{group.id}:{entryId(entry)}"
									disabled={running}
									onSelect={() => select(entry)}
								>
									{@render row(entry)}
								</Command.Item>
							{/if}
						{/each}
					</Command.Group>
				{/each}
			</Command.List>

			<div
				class="flex h-8 flex-none items-center gap-3 border-t border-line-subtle bg-paper-1 px-3 text-xs text-muted-foreground"
			>
				<span class="inline-flex items-center gap-1"><Kbd keys="↑ ↓" />move</span>
				<span class="inline-flex items-center gap-1"><Kbd keys="↵" />{enterHint(listing)}</span>
				<span class="hidden items-center gap-1 sm:inline-flex"><Kbd keys=">" />commands</span>
				<span class="ml-auto truncate">{footNote(listing, scope)}</span>
			</div>
		</Command.Root>
	</Dialog.Content>
</Dialog.Root>

<script lang="ts">
	import { invalidate } from "$app/navigation";
	import { keys } from "$lib/api/keys";
	import { page } from "$app/state";
	import Bot from "@lucide/svelte/icons/bot";
	import ChevronDown from "@lucide/svelte/icons/chevron-down";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import GitBranch from "@lucide/svelte/icons/git-branch";
	import Info from "@lucide/svelte/icons/info";
	import Plug from "@lucide/svelte/icons/plug";
	import Settings from "@lucide/svelte/icons/settings";
	import UserRound from "@lucide/svelte/icons/user-round";
	import Zap from "@lucide/svelte/icons/zap";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Avatar from "$lib/components/ui/avatar/index.js";
	import * as DropdownMenu from "$lib/components/ui/dropdown-menu/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Textarea } from "$lib/components/ui/textarea/index.js";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import Kbd from "$lib/components/norn/kbd.svelte";
	import PriorityIcon from "$lib/components/norn/priority-icon.svelte";
	import StatusIcon from "$lib/components/norn/status-icon.svelte";
	import Tag from "$lib/components/norn/tag.svelte";
	import Toast from "$lib/components/norn/toast.svelte";
	import Markdown from "$lib/issues/markdown.svelte";
	import PropertyPicker from "$lib/issues/property-picker.svelte";
	import { api } from "$lib/api";
	import { bindShortcuts, useShortcuts } from "$lib/shortcuts/registry.svelte";
	import { lift } from "$lib/motion";
	import { priorityLabel } from "$lib/issues/issues";
	import { initialsOf } from "$lib/team/members";
	import { onDateAndTime } from "$lib/time";
	import {
		declineReasons,
		queued,
		readTriageFailure,
		sourceLabels,
		readSource,
		sourceTabs,
		triageFailureMessage,
		type TriageDeclineReason,
		type TriageFailure,
		type TriageListing,
		type TriageSource,
	} from "$lib/triage/triage";
	import type { Issue, IssueCandidate } from "$lib/issues/issues";
	import { workspacePath } from "$lib/workspace/navigation";
	import { triagePreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV ? triagePreviewStates[page.url.searchParams.get("state") ?? ""] : undefined
	);

	let localFailure = $state<TriageFailure | null>(null);
	let working = $state(false);
	let cursor = $state(0);
	const tab = $derived(readSource(page.url.searchParams.get("source")));
	let flow = $state<"accept" | "decline" | "merge" | null>(null);
	let reason = $state<TriageDeclineReason | null>(null);
	let note = $state("");
	let duplicateOf = $state<IssueCandidate | null>(null);
	let notice = $state("");
	let noticeTimer: ReturnType<typeof setTimeout> | undefined;
	let announcement = $state("");
	let pendingTeamId = $state("");

	const slug = $derived(data.workspace.slug);
	const at = $derived((path: string) => workspacePath(slug, path));
	const sourcePath = $derived((source: "all" | TriageSource) =>
		at(source === "all" ? "/triage" : `/triage?source=${source}`)
	);
	const listing = $derived<TriageListing>(preview?.listing ?? data.listing);
	const failure = $derived<TriageFailure | null>(preview?.failure ?? localFailure);
	const teams = $derived(preview?.teams ?? data.teams);
	const members = $derived(data.members ?? []);

	const role = $derived(
		members.find((member) => member.accountId === data.member.id)?.role ?? "member"
	);
	const readOnly = $derived(role === "viewer");

	const waiting = $derived(queued(listing));
	const counts = $derived({
		all: waiting.length,
		user: waiting.filter((issue) => issue.triageSource === "user").length,
		token: waiting.filter((issue) => issue.triageSource === "token").length,
		agent: waiting.filter((issue) => issue.triageSource === "agent").length,
	});
	const shown = $derived(
		tab === "all" ? waiting : waiting.filter((issue) => issue.triageSource === tab)
	);
	const at_cursor = $derived(Math.min(cursor, Math.max(0, shown.length - 1)));
	const item = $derived<Issue | null>(shown[at_cursor] ?? null);

	const teamOf = $derived((issue: Issue) => teams.find((team) => team.id === issue.teamId) ?? null);
	const reporterOf = $derived((issue: Issue) => {
		const member = members.find((candidate) => candidate.accountId === issue.createdByAccountId);

		return member?.displayName || member?.email || sourceLabels[issue.triageSource ?? "user"];
	});

	const duplicates = $derived(
		(data.candidates ?? []).filter(
			(candidate) => candidate.id !== item?.id && candidate.triageState !== "waiting"
		)
	);

	function announce(message: string) {
		notice = message;
		announcement = message;
		clearTimeout(noticeTimer);
		noticeTimer = setTimeout(() => (notice = ""), 5000);
	}

	$effect(() => {
		tab;
		cursor = 0;
		closeFlow();
	});

	function closeFlow() {
		flow = null;
		reason = null;
		note = "";
		duplicateOf = null;
	}

	function move(step: number) {
		closeFlow();
		cursor = Math.min(Math.max(0, at_cursor + step), Math.max(0, shown.length - 1));
	}

	function open(next: "accept" | "decline" | "merge") {
		if (readOnly || !item) return;

		flow = flow === next ? null : next;
		localFailure = null;
	}

	async function decide(
		issue: Issue,
		path: "accept" | "decline" | "merge" | "reassign",
		body?: Record<string, unknown>,
		said?: string
	) {
		working = true;
		localFailure = null;

		try {
			const { error } = await api.POST(
				`/workspaces/{workspaceId}/triage/{issueId}/${path}` as
					"/workspaces/{workspaceId}/triage/{issueId}/accept",
				{
					params: { path: { workspaceId: data.workspace.id, issueId: issue.id } },
					body: body as never,
				}
			);

			if (error) {
				localFailure = readTriageFailure(error);

				return;
			}

			closeFlow();
			announce(said ?? `${issue.reference} decided`);
			await invalidate(keys.triage(data.workspace.id));
		} catch {
			localFailure = { kind: "unavailable" };
		} finally {
			working = false;
		}
	}

	async function accept() {
		if (!item) return;

		await decide(item, "accept", undefined, `${item.reference} is in ${teamOf(item)?.name ?? "the team"}`);
	}

	async function decline() {
		if (!item || !reason) return;

		await decide(
			item,
			"decline",
			{ reason, ...(note.trim() ? { note: note.trim() } : {}) },
			`Declined ${item.reference}`
		);
	}

	async function merge() {
		if (!item || !duplicateOf) return;

		await decide(
			item,
			"merge",
			{ duplicateOfId: duplicateOf.id },
			`Merged ${item.reference} into ${duplicateOf.reference}`
		);
	}

	async function sendTo(teamId: string, acknowledgeLabelLoss = false) {
		if (!item) return;

		const team = teams.find((candidate) => candidate.id === teamId);

		pendingTeamId = teamId;

		await decide(
			item,
			"reassign",
			{ teamId, expectedVersion: item.version, acknowledgeLabelLoss },
			`Sent ${item.reference} to ${team?.name ?? "the team"}`
		);
	}

	const shortcuts = useShortcuts();

	bindShortcuts({
		"triage-close": closeFlow,
		"cursor-down": () => move(1),
		"cursor-up": () => move(-1),
	});

	$effect(() => {
		if (readOnly || !item) return;

		const released = [
			shortcuts.register("triage-accept", () => open("accept")),
			shortcuts.register("triage-decline", () => open("decline")),
			shortcuts.register("triage-move", () => open("merge")),
		];

		return () => released.forEach((release) => release());
	});

	const sourceGlyph = { user: UserRound, token: Plug, agent: Bot };
</script>

{#snippet sourceMark(source: TriageSource)}
	{@const Glyph = sourceGlyph[source]}
	<Glyph class="size-3.25" aria-hidden="true" />
{/snippet}

<svelte:head><title>Triage · {data.workspace.name} · Norn</title></svelte:head>

<div class="flex min-h-0 flex-1 flex-col">
	<div class="flex-none border-b border-line-default">
		<div class="flex h-11 items-center gap-2 pr-3 pl-4">
			<Zap class="size-icon-toolbar shrink-0 text-muted-foreground" aria-hidden="true" />
			<h1 class="text-md font-medium tracking-snug whitespace-nowrap text-ink-900">Triage</h1>
			<span class="font-mono text-xs text-muted-foreground">{counts.all}</span>
			{#if item}
				<span class="h-3.5 w-px bg-line-default" aria-hidden="true"></span>
				<span class="font-mono text-xs text-muted-foreground">
					{at_cursor + 1} of {shown.length}
				</span>
			{/if}
			<div class="flex-1"></div>
			<Button variant="ghost" size="sm" href={at("/settings/teams")}>
				<Settings aria-hidden="true" />
				Triage rules
			</Button>
		</div>

		<div class="flex h-8.5 items-center gap-2.5 border-t border-line-subtle pr-3 pl-3.5">
			<div
				class="flex min-w-0 gap-0.5 overflow-x-auto rounded-sm border border-line-default p-0.5"
			>
				{#each sourceTabs as choice (choice.value)}
					<a
						href={sourcePath(choice.value)}
						aria-current={tab === choice.value ? "page" : undefined}
						class="inline-flex h-5.5 flex-none cursor-pointer items-center gap-1.5 rounded-xs px-2 font-mono text-2xs tracking-eyebrow uppercase motion-control {tab ===
						choice.value
							? 'bg-primary text-primary-foreground'
							: 'text-ink-600 hover:bg-accent'}"
					>
						{choice.label}
						<span class="tabular-nums opacity-70">{counts[choice.value]}</span>
					</a>
				{/each}
			</div>
			<div class="flex-1"></div>
			{#if readOnly}
				<span class="inline-flex items-center gap-1.5 font-mono text-2xs tracking-eyebrow text-muted-foreground uppercase">
					<Info class="size-3" aria-hidden="true" />
					Read only
				</span>
			{/if}
			<span class="hidden text-sm whitespace-nowrap text-muted-foreground sm:inline">
				Oldest first
			</span>
		</div>
	</div>

	<p class="sr-only" role="status" aria-live="polite">{announcement}</p>

	{#if failure}
		<div class="flex-none px-4 pt-3">
			<Alert.Root variant="destructive">
				<CircleX aria-hidden="true" />
				<Alert.Title>That did not stick</Alert.Title>
				<Alert.Description>{triageFailureMessage(failure)}</Alert.Description>
			</Alert.Root>
		</div>
	{/if}

	{#if listing.kind === "loading"}
		<div class="flex min-h-0 flex-1">
			<div class="w-81.5 flex-none border-r border-line-default p-3" aria-busy="true">
				{#each [1, 2, 3, 4, 5] as row (row)}
					<div class="mb-3 flex flex-col gap-2">
						<span class="block h-3 w-4/5 animate-breathe rounded-xs bg-paper-2"></span>
						<span class="block h-2.5 w-2/5 animate-breathe rounded-xs bg-paper-2"></span>
					</div>
				{/each}
			</div>
			<div class="flex-1 p-7">
				<span class="mb-4 block h-5 w-1/2 animate-breathe rounded-xs bg-paper-3"></span>
				<span class="mb-2 block h-3 w-full animate-breathe rounded-xs bg-paper-2"></span>
				<span class="block h-3 w-4/5 animate-breathe rounded-xs bg-paper-2"></span>
			</div>
		</div>
	{:else if listing.kind === "unavailable"}
		<div class="flex flex-1 items-start justify-center p-10">
			<Alert.Root variant="destructive" class="max-w-120">
				<CircleX aria-hidden="true" />
				<Alert.Title>We could not read the queue</Alert.Title>
				<Alert.Description>Nothing changed. Wait a moment and try again.</Alert.Description>
			</Alert.Root>
		</div>
	{:else if shown.length === 0}
		<div class="flex flex-1 flex-col items-center justify-center gap-5 p-10">
			<div class="flex max-w-95 flex-col items-center gap-2.5 text-center">
				<Zap class="size-4.5 text-muted-foreground" aria-hidden="true" />
				<span class="font-mono text-xs tracking-eyebrow text-ink-600 uppercase">Triage zero</span>
				<p class="text-md leading-normal text-muted-foreground text-pretty">
					Reports from integrations, agents and people outside the team land here before anyone
					commits to them.
				</p>
				<div class="mt-1">
					<Button variant="secondary" size="sm" href={at("/issues")}>Back to issues</Button>
				</div>
			</div>
		</div>
	{:else}
		<div class="flex min-h-0 flex-1 flex-col lg:flex-row">
			<div
				class="flex max-h-72 min-h-0 w-full flex-none flex-col overflow-auto border-b border-line-default lg:max-h-none lg:w-81.5 lg:border-r lg:border-b-0"
			>
				<ul>
					{#each shown as issue, index (issue.id)}
						{@const Glyph = sourceGlyph[issue.triageSource ?? "user"]}
						<li>
							<button
								type="button"
								aria-current={index === at_cursor}
								onclick={() => {
									cursor = index;
									closeFlow();
								}}
								class="flex w-full cursor-pointer gap-2.25 border-b border-line-subtle px-3 py-2.25 text-left motion-control hover:bg-accent {index ===
								at_cursor
									? 'rule-inset bg-surface-cursor'
									: ''}"
							>
								<Glyph
									class="mt-0.25 size-icon-row shrink-0 {index === at_cursor
										? 'text-ink-900'
										: 'text-muted-foreground'}"
									aria-hidden="true"
								/>
								<span class="flex min-w-0 flex-1 flex-col gap-1">
									<span
										class="truncate text-md tracking-snug {index === at_cursor
											? 'text-ink-900'
											: 'text-ink-600'}"
									>
										{issue.title}
									</span>
									<span
										class="flex items-center gap-1.75 font-mono text-2xs text-muted-foreground"
									>
										<span>{sourceLabels[issue.triageSource ?? "user"]}</span>
										<span class="size-0.5 rounded-full bg-line-strong" aria-hidden="true"></span>
										<span>{teamOf(issue)?.key ?? issue.teamKey}</span>
										{#if issue.priority === "urgent"}
											<span class="text-priority-urgent">Urgent</span>
										{/if}
									</span>
								</span>
							</button>
						</li>
					{/each}
				</ul>
				<p class="px-3.5 py-3 font-mono text-2xs tracking-eyebrow text-muted-foreground uppercase">
					{shown.length} waiting
				</p>
			</div>

			{#if item}
				<div class="relative flex min-h-0 min-w-0 flex-1 flex-col">
					<div class="flex-1 overflow-auto px-5 pt-6 pb-5 sm:px-7">
						<div class="flex max-w-180 flex-col gap-5">
							{#if readOnly}
								<Alert.Root variant="muted">
									<Info aria-hidden="true" />
									<Alert.Title>You can read the queue but not act on it</Alert.Title>
									<Alert.Description>
										Accepting, declining and merging need more than read access. Ask a workspace
										admin.
									</Alert.Description>
								</Alert.Root>
							{/if}

							<div class="flex flex-col gap-2.5">
								<div class="flex flex-wrap items-center gap-2">
									<span
										class="inline-flex items-center gap-1.5 font-mono text-2xs font-medium tracking-eyebrow text-ink-600 uppercase"
									>
										{@render sourceMark(item.triageSource ?? "user")}
										{sourceLabels[item.triageSource ?? "user"]}
									</span>
									<span class="h-3 w-px bg-line-default" aria-hidden="true"></span>
									<span class="font-mono text-xs text-muted-foreground">
										received <time datetime={item.createdAt}>
											{onDateAndTime(item.createdAt, data.workspace.timezone)}
										</time>
									</span>
									<div class="flex-1"></div>
									<Button variant="ghost" size="sm" href={at(`/issues/${item.reference}`)}>
										Open {item.reference}
									</Button>
								</div>

								<h2
									class="text-xl leading-tight font-medium tracking-title text-ink-900 text-pretty"
								>
									{item.title}
								</h2>

								<div class="flex flex-wrap items-center gap-2">
									<Avatar.Root size="xs">
										<Avatar.Fallback>{initialsOf(reporterOf(item))}</Avatar.Fallback>
									</Avatar.Root>
									<span class="text-sm text-ink-600">{reporterOf(item)}</span>
									<span class="font-mono text-xs text-muted-foreground">
										{item.createdByAccountId ? "member" : "outside the workspace"}
									</span>
								</div>
							</div>

							{#if item.description.trim()}
								<Markdown source={item.description} />
							{:else}
								<p class="text-md text-muted-foreground">Nothing was written with it.</p>
							{/if}

							<div class="flex flex-col gap-2">
								<Eyebrow rule class="text-ink-600">Context</Eyebrow>
								<dl
									class="grid grid-cols-[8rem_1fr] gap-x-4 gap-y-1.75 rounded-lg border border-line-default bg-card px-3.5 py-3"
								>
									<dt class="font-mono text-xs text-muted-foreground">Team</dt>
									<dd class="text-sm text-ink-900">
										{teamOf(item)?.name ?? item.teamKey}
									</dd>

									<dt class="font-mono text-xs text-muted-foreground">Priority</dt>
									<dd class="flex items-center gap-1.5 text-sm text-ink-900">
										<PriorityIcon priority={item.priority} />
										{priorityLabel(item.priority)}
									</dd>

									<dt class="font-mono text-xs text-muted-foreground">State</dt>
									<dd class="flex items-center gap-1.5 text-sm text-ink-900">
										<StatusIcon category={item.state.category} decorative />
										{item.state.name}
									</dd>

									{#if item.labels.length > 0}
										<dt class="font-mono text-xs text-muted-foreground">Labels</dt>
										<dd class="flex flex-wrap gap-1.5">
											{#each item.labels as label (label.id)}
												<Tag name={label.name} color={label.color} />
											{/each}
										</dd>
									{/if}

									<dt class="font-mono text-xs text-muted-foreground">Reference</dt>
									<dd class="font-mono text-sm text-ink-900">{item.reference}</dd>
								</dl>
							</div>
						</div>
					</div>

					{#if flow === "accept"}
						<div
							class="notch absolute right-4 bottom-14 left-4 z-50 max-w-130 sm:left-7"
							transition:lift
						>
							<div class="flex flex-col gap-3 p-3.5">
								<Eyebrow class="text-ink-600">Accept as issue</Eyebrow>
								<p class="text-md text-muted-foreground text-pretty">
									{item.reference} joins {teamOf(item)?.name ?? item.teamKey} in
									{item.state.name}. Everything on it stays as it is, and whoever reported it can
									follow along.
								</p>
								<div class="flex items-center gap-2">
									<div class="flex-1"></div>
									<Button variant="ghost" size="sm" onclick={closeFlow}>Cancel</Button>
									<Button size="sm" disabled={working} onclick={accept}>
										{working ? "Accepting" : `Accept ${item.reference}`}
									</Button>
								</div>
							</div>
						</div>
					{/if}

					{#if flow === "decline"}
						<div
							class="notch absolute right-4 bottom-14 left-4 z-50 max-w-130 sm:left-7"
							transition:lift
						>
							<div class="flex flex-col gap-2.5 p-3.5">
								<Eyebrow class="text-ink-600">Decline · pick a reason</Eyebrow>
								<div class="flex flex-col gap-0.5">
									{#each declineReasons as choice (choice.value)}
										<button
											type="button"
											aria-pressed={reason === choice.value}
											onclick={() => (reason = choice.value)}
											class="flex h-7 w-full cursor-pointer items-center gap-2 rounded-sm px-1.5 text-left text-md motion-control hover:bg-accent {reason ===
											choice.value
												? 'bg-surface-selected text-ink-900'
												: 'text-ink-600'}"
										>
											{choice.label}
										</button>
									{/each}
								</div>
								<Textarea
									bind:value={note}
									rows={2}
									placeholder="Note to the reporter (optional)"
									class="min-h-14 resize-none"
								/>
								<div class="flex items-center gap-2">
									<span class="flex-1 text-sm text-muted-foreground">
										{reason
											? "The reason is kept in its history, and it closes."
											: "Pick a reason first."}
									</span>
									<Button variant="ghost" size="sm" onclick={closeFlow}>Cancel</Button>
									<Button variant="secondary" size="sm" disabled={!reason || working} onclick={decline}>
										{working ? "Declining" : "Decline"}
									</Button>
								</div>
							</div>
						</div>
					{/if}

					{#if flow === "merge"}
						<div
							class="notch absolute right-4 bottom-14 left-4 z-50 max-w-130 sm:left-7"
							transition:lift
						>
							<div class="flex flex-col gap-2.5 p-3.5">
								<Eyebrow class="text-ink-600">Mark as duplicate · choose the original</Eyebrow>
								<PropertyPicker
									options={duplicates.slice(0, 80).map((candidate) => ({
										value: candidate.id,
										label: `${candidate.reference} ${candidate.title}`,
										checked: candidate.id === duplicateOf?.id,
									}))}
									placeholder="Search issues…"
									empty="No issue matches that"
									onpick={(id) =>
										(duplicateOf = duplicates.find((candidate) => candidate.id === id) ?? null)}
								>
									{#snippet trigger(props)}
										<button
											{...props}
											type="button"
											class="flex h-control-md w-full cursor-pointer items-center gap-2 rounded-md border border-line-default bg-paper-0 px-2.5 text-left text-md text-ink-900 motion-control hover:bg-accent aria-expanded:bg-accent"
										>
											{#if duplicateOf}
												<StatusIcon category={duplicateOf.state.category} decorative />
												<span class="font-mono text-xs text-muted-foreground">
													{duplicateOf.reference}
												</span>
												<span class="min-w-0 flex-1 truncate">{duplicateOf.title}</span>
											{:else}
												<span class="min-w-0 flex-1 truncate text-muted-foreground">
													Search issues…
												</span>
											{/if}
											<ChevronDown class="size-icon-row text-muted-foreground" aria-hidden="true" />
										</button>
									{/snippet}
								</PropertyPicker>
								<div class="flex items-center gap-2">
									<span class="flex-1 text-sm text-muted-foreground">
										{duplicateOf
											? `${item.reference} closes as a duplicate and links to ${duplicateOf.reference}.`
											: "Choose the issue this duplicates."}
									</span>
									<Button variant="ghost" size="sm" onclick={closeFlow}>Cancel</Button>
									<Button size="sm" disabled={!duplicateOf || working} onclick={merge}>
										{duplicateOf ? `Merge into ${duplicateOf.reference}` : "Merge"}
									</Button>
								</div>
							</div>
						</div>
					{/if}

					<div
						class="flex h-12 flex-none flex-wrap items-center gap-2 border-t border-line-default px-4 sm:px-7"
					>
						<Button size="sm" disabled={readOnly || working} onclick={() => open("accept")}>
							Accept
							<Kbd keys="A" tone="inverse" />
						</Button>
						<Button
							variant="secondary"
							size="sm"
							disabled={readOnly || working}
							onclick={() => open("decline")}
						>
							Decline
							<Kbd keys="D" />
						</Button>
						<Button
							variant="ghost"
							size="sm"
							disabled={readOnly || working}
							onclick={() => open("merge")}
						>
							<GitBranch aria-hidden="true" />
							Duplicate
							<Kbd keys="M" />
						</Button>

						<DropdownMenu.Root>
							<DropdownMenu.Trigger disabled={readOnly || working}>
								{#snippet child({ props })}
									<Button {...props} variant="ghost" size="sm" disabled={readOnly || working}>
										Send to team
										<ChevronDown aria-hidden="true" />
									</Button>
								{/snippet}
							</DropdownMenu.Trigger>
							<DropdownMenu.Content align="start" class="max-h-80 overflow-auto">
								<DropdownMenu.Group>
									<DropdownMenu.GroupHeading>Send to</DropdownMenu.GroupHeading>
									{#each teams.filter((team) => team.id !== item.teamId) as team (team.id)}
										<DropdownMenu.Item onSelect={() => sendTo(team.id)}>
											{team.key} · {team.name}
										</DropdownMenu.Item>
									{/each}
								</DropdownMenu.Group>
							</DropdownMenu.Content>
						</DropdownMenu.Root>

						<div class="flex-1"></div>

						<span class="hidden items-center gap-1.5 text-xs text-muted-foreground sm:inline-flex">
							<Kbd keys="J K" /> move through the queue
						</span>
						<span class="font-mono text-xs text-muted-foreground">
							{at_cursor + 1} of {shown.length}
						</span>
					</div>
				</div>
			{/if}
		</div>
	{/if}
</div>

{#if notice}
	<div class="fixed right-4 bottom-4 z-70">
		<Toast message={notice} />
	</div>
{/if}

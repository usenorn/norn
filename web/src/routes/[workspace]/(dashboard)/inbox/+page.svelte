<script lang="ts">
	import { invalidate } from "$app/navigation";
	import { keys } from "$lib/api/keys";
	import { page } from "$app/state";
	import Bot from "@lucide/svelte/icons/bot";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import Clock from "@lucide/svelte/icons/clock";
	import Cog from "@lucide/svelte/icons/cog";
	import Inbox from "@lucide/svelte/icons/inbox";
	import Plug from "@lucide/svelte/icons/plug";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as DropdownMenu from "$lib/components/ui/dropdown-menu/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { api } from "$lib/api";
	import { goto } from "$app/navigation";
	import { listCursor } from "$lib/shortcuts/list-cursor.svelte";
	import ShortcutBar from "$lib/shortcuts/shortcut-bar.svelte";
	import {
		actorKindLabels,
		listingFor,
		notificationFailureMessage,
		readNotificationFailure,
		reasonLabels,
		subjectPath,
		summary,
		type InboxListing,
		type Notification,
		type NotificationFailure,
	} from "$lib/notifications/notifications";
	import { onDateAndTime } from "$lib/time";
	import { workspacePath } from "$lib/workspace/navigation";
	import { inboxPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV ? inboxPreviewStates[page.url.searchParams.get("state") ?? ""] : undefined
	);

	let localFailure = $state<NotificationFailure | null>(null);
	let working = $state("");
	let extra = $state.raw<Notification[]>([]);
	let pageCursor = $state<string | undefined>(undefined);
	let loadingMore = $state(false);
	let announcement = $state("");

	const slug = $derived(data.workspace.slug);
	const filter = $derived(preview?.filter ?? data.filter);
	const unread = $derived(preview?.unread ?? data.unread);
	const failure = $derived<NotificationFailure | null>(preview?.failure ?? localFailure);
	const busy = $derived(working !== "");

	const listing = $derived<InboxListing>(
		mergeLoaded(preview?.listing ?? data.listing, extra, pageCursor)
	);

	const rows = $derived(listing.kind === "ready" ? listing.notifications : []);

	const cursor = listCursor(() => ({
		rows,
		open: (notification) => void goto(subjectPath(slug, notification)),
	}));

	function mergeLoaded(
		base: InboxListing,
		loaded: Notification[],
		next: string | undefined
	): InboxListing {
		if (base.kind !== "ready" || loaded.length === 0) return base;

		return { kind: "ready", notifications: [...base.notifications, ...loaded], nextCursor: next };
	}

	function marker(notification: Notification) {
		if (notification.actorKind === "agent") return Bot;
		if (notification.actorKind === "token") return Plug;
		if (notification.actorKind === "system") return Cog;

		return null;
	}

	function key(notification: Notification): string {
		return `${notification.subjectKind}:${notification.subjectId}`;
	}

	async function markRead(notification: Notification) {
		working = key(notification);
		localFailure = null;

		try {
			const { error } = await api.POST(
				"/workspaces/{workspaceId}/notifications/{subjectKind}/{subjectId}/read",
				{
					params: {
						path: {
							workspaceId: data.workspace.id,
							subjectKind: notification.subjectKind,
							subjectId: notification.subjectId,
						},
					},
				}
			);

			if (error) {
				localFailure = readNotificationFailure(error);

				return;
			}

			announcement = `${notification.title} is marked read.`;
			await reload();
		} catch {
			localFailure = { kind: "unavailable" };
		} finally {
			working = "";
		}
	}

	async function markAllRead() {
		working = "all";
		localFailure = null;

		try {
			const { error } = await api.POST("/workspaces/{workspaceId}/notifications/read", {
				params: { path: { workspaceId: data.workspace.id } },
			});

			if (error) {
				localFailure = readNotificationFailure(error);

				return;
			}

			announcement = "Your inbox is clear.";
			await reload();
		} catch {
			localFailure = { kind: "unavailable" };
		} finally {
			working = "";
		}
	}

	async function snooze(notification: Notification, hours: number) {
		working = key(notification);
		localFailure = null;

		try {
			const until = new Date(Date.now() + hours * 3600_000).toISOString();

			const { error } = await api.POST(
				"/workspaces/{workspaceId}/notifications/{subjectKind}/{subjectId}/snooze",
				{
					params: {
						path: {
							workspaceId: data.workspace.id,
							subjectKind: notification.subjectKind,
							subjectId: notification.subjectId,
						},
					},
					body: { until },
				}
			);

			if (error) {
				localFailure = readNotificationFailure(error);

				return;
			}

			announcement = `${notification.title} is hidden for now.`;
			await reload();
		} catch {
			localFailure = { kind: "unavailable" };
		} finally {
			working = "";
		}
	}

	async function loadMore() {
		if (listing.kind !== "ready" || !listing.nextCursor || loadingMore) return;

		loadingMore = true;
		localFailure = null;

		try {
			const { data: next, error } = await api.GET("/workspaces/{workspaceId}/notifications", {
				params: {
					path: { workspaceId: data.workspace.id },
					query: { filter, cursor: listing.nextCursor },
				},
			});

			if (error || !next) {
				localFailure = { kind: "unavailable" };

				return;
			}

			extra = [...extra, ...next.notifications];
			pageCursor = next.nextCursor;
		} catch {
			localFailure = { kind: "unavailable" };
		} finally {
			loadingMore = false;
		}
	}

	async function reload() {
		extra = [];
		pageCursor = undefined;
		await invalidate(keys.inbox(data.workspace.id));
	}
</script>

<svelte:head><title>Inbox · {data.workspace.name} · Norn</title></svelte:head>

<div class="flex min-h-0 flex-1 flex-col">
	<div class="flex-none border-b border-line-default">
		<div class="flex h-11 flex-wrap items-center gap-2 pr-3 pl-4">
			<Inbox class="size-icon-toolbar shrink-0 text-muted-foreground" aria-hidden="true" />
			<h1 class="text-md font-medium tracking-snug whitespace-nowrap text-ink-900">Inbox</h1>
			<span class="text-sm text-muted-foreground">
				{unread === 0 ? "Nothing unread" : `${unread} unread`}
			</span>
			<div class="ml-auto flex shrink-0 items-center gap-1">
				<Button
					href={workspacePath(slug, `/inbox?filter=${filter === "unread" ? "all" : "unread"}`)}
					variant="ghost"
					size="sm"
				>
					{filter === "unread" ? "Show everything" : "Show unread only"}
				</Button>
				<Button
					variant="secondary"
					size="sm"
					disabled={busy || unread === 0}
					onclick={markAllRead}
				>
					Mark all read
				</Button>
			</div>
		</div>
	</div>

	<div class="flex-1 overflow-auto">
		<div class="pb-[calc(--spacing(10)+env(safe-area-inset-bottom))]">
			<p class="sr-only" role="status" aria-live="polite">{announcement}</p>

			{#if failure}
				<div class="px-4 pt-3">
					<Alert.Root variant="destructive">
						<CircleX aria-hidden="true" />
						<Alert.Title>That did not stick</Alert.Title>
						<Alert.Description>{notificationFailureMessage(failure)}</Alert.Description>
					</Alert.Root>
				</div>
			{/if}

			{#if listing.kind === "loading"}
				<div class="m-4 h-40 animate-breathe rounded-lg bg-paper-2" aria-busy="true"></div>
			{:else if listing.kind === "unavailable"}
				<p class="p-6 text-md leading-normal text-muted-foreground">
					We could not load your inbox. Nothing changed &mdash; wait a moment and try again.
				</p>
			{:else if listing.kind === "caught_up"}
				<div class="flex flex-col items-center gap-3 px-6 py-16 text-center">
					<p class="text-md font-medium tracking-snug text-ink-900">You are caught up</p>
					<p class="max-w-90 text-md leading-normal text-muted-foreground text-pretty">
						Twelve changes to one issue arrive as one line here, so this stays worth reading.
					</p>
					<Button href={workspacePath(slug, "/inbox?filter=all")} variant="secondary" size="sm">
						Show everything
					</Button>
				</div>
			{:else if listing.kind === "empty"}
				<div class="flex flex-col items-center gap-3 px-6 py-16 text-center">
					<p class="text-md font-medium tracking-snug text-ink-900">Nothing here yet</p>
					<p class="max-w-90 text-md leading-normal text-muted-foreground text-pretty">
						Being assigned something, being mentioned, or following an issue puts it here.
					</p>
					<Button href={workspacePath(slug, "/my-tasks")} variant="secondary" size="sm">
						Go to your tasks
					</Button>
				</div>
			{:else}
				<ul>
					{#each listing.notifications as notification (key(notification))}
						{@const Marker = marker(notification)}
						{@const snoozed = notification.snoozedUntil}
						<li class="cursor-row border-b border-line-subtle" {...cursor.props(notification)}>
							<div class="flex flex-wrap items-center gap-2 pr-3">
								<a
									href={subjectPath(slug, notification)}
									class="flex min-w-0 flex-[1_1_240px] items-center gap-2 py-2 pl-4 focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-ring"
								>
									<span
										class="size-1.5 shrink-0 rounded-full {notification.unreadCount > 0
											? 'bg-status-active'
											: 'bg-transparent'}"
										aria-hidden="true"
									></span>
									<span class="min-w-0 flex-1">
										<span class="flex min-w-0 items-center gap-2">
											{#if notification.reference}
												<span class="shrink-0 font-mono text-2xs text-muted-foreground">
													{notification.reference}
												</span>
											{/if}
											<span
												class="min-w-0 truncate text-md tracking-snug {notification.unreadCount >
												0
													? 'font-medium text-ink-900'
													: 'text-ink-600'}"
											>
												{notification.title}
											</span>
										</span>
										<span class="flex min-w-0 items-center gap-1.5 text-sm text-muted-foreground">
											{#if Marker}
												<Marker
													class="size-3 shrink-0"
													aria-label={actorKindLabels[notification.actorKind]}
												/>
											{/if}
											<span class="min-w-0 truncate">{summary(notification)}</span>
										</span>
									</span>
								</a>

								<span
									class="shrink-0 rounded-sm border border-line-default px-1.5 py-0.5 text-sm whitespace-nowrap text-muted-foreground"
								>
									{reasonLabels[notification.reason]}
								</span>

								<time
									class="shrink-0 text-sm whitespace-nowrap text-muted-foreground"
									datetime={notification.lastEventAt}
								>
									{onDateAndTime(notification.lastEventAt, data.workspace.timezone)}
								</time>

								{#if snoozed}
									<span
										class="flex shrink-0 items-center gap-1 text-sm whitespace-nowrap text-muted-foreground"
									>
										<Clock class="size-3" aria-hidden="true" />
										Until {onDateAndTime(snoozed, data.workspace.timezone)}
									</span>
								{/if}

								<div class="flex shrink-0 items-center gap-1">
									<Button
										size="sm"
										variant="secondary"
										disabled={busy || notification.unreadCount === 0}
										onclick={() => markRead(notification)}
									>
										Mark read
									</Button>
									<DropdownMenu.Root>
										<DropdownMenu.Trigger disabled={busy}>
											{#snippet child({ props })}
												<Button {...props} size="sm" variant="ghost">Snooze</Button>
											{/snippet}
										</DropdownMenu.Trigger>
										<DropdownMenu.Content align="end">
											<DropdownMenu.Item onSelect={() => snooze(notification, 1)}>
												For an hour
											</DropdownMenu.Item>
											<DropdownMenu.Item onSelect={() => snooze(notification, 8)}>
												Until later today
											</DropdownMenu.Item>
											<DropdownMenu.Item onSelect={() => snooze(notification, 24)}>
												Until tomorrow
											</DropdownMenu.Item>
											<DropdownMenu.Item onSelect={() => snooze(notification, 168)}>
												Until next week
											</DropdownMenu.Item>
										</DropdownMenu.Content>
									</DropdownMenu.Root>
								</div>
							</div>
						</li>
					{/each}
				</ul>

				{#if listing.nextCursor}
					<div class="flex justify-center px-4 py-3">
						<Button variant="secondary" size="sm" disabled={loadingMore} onclick={loadMore}>
							{loadingMore ? "Loading…" : "Load more"}
						</Button>
					</div>
				{/if}
			{/if}
		</div>
	</div>

	<ShortcutBar ids={["cursor-down", "cursor-open", "issue-new", "search", "help"]} />
</div>

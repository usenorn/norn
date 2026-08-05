<script lang="ts">
	import { goto } from "$app/navigation";
	import { page } from "$app/state";
	import CircleCheck from "@lucide/svelte/icons/circle-check";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import Plug from "@lucide/svelte/icons/plug";
	import UserRound from "@lucide/svelte/icons/user-round";
	import X from "@lucide/svelte/icons/x";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Avatar from "$lib/components/ui/avatar/index.js";
	import * as Select from "$lib/components/ui/select/index.js";
	import Tag from "$lib/components/norn/tag.svelte";
	import TeamKey from "$lib/components/norn/team-key.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { api } from "$lib/api";
	import { initialsOf } from "$lib/team/members";
	import {
		authMethodLabel,
		hasMore,
		isDirectoryManaged,
		joinedOn,
		lastActive,
		memberName,
		memberPageSize,
		membersOf,
		membershipFailureMessage,
		membershipFailureTitle,
		membershipRoles,
		removalFailureFor,
		roleFailureFor,
		roleLabels,
		roleNotes,
		searchDebounceMs,
		type MemberAction,
		type MemberListing,
		type MemberPaging,
		type MemberRemoval,
		type Membership,
		type MembershipFailure,
		type MembershipNotice,
		type MembershipRole,
	} from "$lib/workspace/members";
	import { workspacePath } from "$lib/workspace/navigation";
	import { membersPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	type Accumulated = { source: MemberListing; members: Membership[]; nextCursor?: string };

	let { data }: PageProps = $props();

	const linked = $derived(new Set(data.linked ?? []));

	const preview = $derived(
		import.meta.env.DEV ? membersPreviewStates[page.url.searchParams.get("state") ?? ""] : undefined
	);

	let accumulated = $state.raw<Accumulated | null>(null);
	let localPaging = $state<MemberPaging>({ kind: "idle" });
	let localRemoval = $state<MemberRemoval>({ kind: "closed" });
	let localAction = $state<MemberAction>({ kind: "idle" });
	let localFailure = $state<MembershipFailure | null>(null);
	let localNotice = $state<MembershipNotice | null>(null);
	let reassignTo = $state("");
	let announcement = $state("");

	let draft = $state(page.url.searchParams.get("q") ?? "");
	let pushed = page.url.searchParams.get("q") ?? "";
	let debounce: ReturnType<typeof setTimeout> | undefined;

	const slug = $derived(page.params.workspace ?? "");
	const workspace = $derived(data.workspace);
	const viewerId = $derived(preview?.viewerId ?? data.member.id);
	const committed = $derived(page.url.searchParams.get("q") ?? "");

	const base = $derived<MemberListing>(preview?.listing ?? data.listing);
	const listing = $derived<MemberListing>(
		accumulated && accumulated.source === base && base.kind === "results"
			? { kind: "results", members: accumulated.members, nextCursor: accumulated.nextCursor }
			: base
	);

	const members = $derived(membersOf(listing));
	const paging = $derived<MemberPaging>(preview?.paging ?? localPaging);
	const action = $derived<MemberAction>(preview?.action ?? localAction);
	const failure = $derived<MembershipFailure | null>(preview?.failure ?? localFailure);
	const notice = $derived<MembershipNotice | null>(preview?.notice ?? localNotice);
	const busy = $derived(action.kind !== "idle" || localRemoval.kind === "removing");

	let revokedTokens = $state<string[]>([]);
	let revokingToken = $state<string | null>(null);

	const tokens = $derived(
		data.tokens?.filter((owned) => !revokedTokens.includes(owned.token.id)) ?? null
	);

	async function revokeToken(tokenId: string) {
		revokingToken = tokenId;

		try {
			const { error } = await api.DELETE("/workspaces/{workspaceId}/tokens/{tokenId}", {
				params: { path: { workspaceId: workspace.id, tokenId } },
			});

			if (!error) revokedTokens = [...revokedTokens, tokenId];
		} finally {
			revokingToken = null;
		}
	}

	const removalId = $derived(page.url.searchParams.get("remove"));
	const removal = $derived<MemberRemoval>(preview?.removal ?? localRemoval);
	const removalMember = $derived("member" in removal ? removal.member : null);

	const directoryInView = $derived(members.some(isDirectoryManaged));

	$effect(() => {
		if (committed === pushed) return;

		pushed = committed;
		draft = committed;
	});

	$effect(() => () => clearTimeout(debounce));

	$effect(() => {
		const id = removalId;

		if (!id) {
			localRemoval = { kind: "closed" };
			reassignTo = "";

			return;
		}

		if (localRemoval.kind !== "closed" && "accountId" in localRemoval && localRemoval.accountId === id) {
			return;
		}

		if (removalMember?.accountId === id) return;

		openRemoval(id);
	});

	async function commit(value: string) {
		pushed = value;

		const next = new URL(page.url);

		if (value) next.searchParams.set("q", value);
		else next.searchParams.delete("q");

		next.searchParams.delete("remove");

		await goto(next, { replaceState: true, keepFocus: true, noScroll: true });
	}

	function type(value: string) {
		draft = value;
		clearTimeout(debounce);
		debounce = setTimeout(() => commit(value), searchDebounceMs);
	}

	function flush(event: KeyboardEvent) {
		if (event.key !== "Enter") return;

		event.preventDefault();
		clearTimeout(debounce);
		commit(draft);
	}

	async function loadMore() {
		if (listing.kind !== "results" || !listing.nextCursor) return;

		localPaging = { kind: "loading" };

		try {
			const { data: next } = await api.GET("/workspaces/{workspaceId}/members", {
				params: {
					path: { workspaceId: workspace.id },
					query: { query: committed || undefined, limit: memberPageSize, cursor: listing.nextCursor },
				},
			});

			if (!next) {
				localPaging = { kind: "unavailable" };

				return;
			}

			accumulated = {
				source: base,
				members: [...members, ...next.members],
				nextCursor: next.nextCursor,
			};
			localPaging = { kind: "idle" };
			announcement = `${next.members.length} more people loaded.`;
		} catch {
			localPaging = { kind: "unavailable" };
		}
	}

	function replace(next: Membership[]) {
		accumulated = {
			source: base,
			members: next,
			nextCursor: listing.kind === "results" ? listing.nextCursor : undefined,
		};
	}

	async function changeRole(member: Membership, role: MembershipRole) {
		if (role === member.role) return;

		localAction = { kind: "changing_role", accountId: member.accountId };
		localFailure = null;
		localNotice = null;

		try {
			const { data: updated, error } = await api.PATCH(
				"/workspaces/{workspaceId}/members/{accountId}",
				{
					params: { path: { workspaceId: workspace.id, accountId: member.accountId } },
					body: { role },
				}
			);

			if (updated) {
				replace(members.map((row) => (row.accountId === updated.accountId ? updated : row)));
				localNotice = { kind: "role_changed", name: memberName(updated), role };
				announcement = `${memberName(updated)} is now ${roleLabels[role]}.`;

				return;
			}

			localFailure = roleFailureFor(error, member);
		} catch {
			localFailure = { kind: "unavailable" };
		} finally {
			localAction = { kind: "idle" };
		}
	}

	async function openRemoval(accountId: string) {
		const known = members.find((row) => row.accountId === accountId);

		if (!known) {
			localRemoval = { kind: "not_in_view", accountId };

			return;
		}

		localRemoval = { kind: "loading", accountId };

		try {
			const { data: preflight } = await api.GET(
				"/workspaces/{workspaceId}/members/{accountId}/removal",
				{ params: { path: { workspaceId: workspace.id, accountId } } }
			);

			if (!preflight) {
				localRemoval = { kind: "unavailable", accountId };

				return;
			}

			localRemoval = {
				kind: "ready",
				member: preflight.member,
				teams: preflight.teams,
				soleAdmin: preflight.soleAdmin,
			};
		} catch {
			localRemoval = { kind: "unavailable", accountId };
		}
	}

	async function confirmRemoval() {
		if (removal.kind !== "ready") return;

		const member = removal.member;
		const target = members.find((row) => row.accountId === reassignTo);

		localRemoval = { ...removal, kind: "removing" };
		localFailure = null;
		localNotice = null;

		try {
			const { error } = await api.DELETE("/workspaces/{workspaceId}/members/{accountId}", {
				params: {
					path: { workspaceId: workspace.id, accountId: member.accountId },
					query: reassignTo ? { reassignTo } : {},
				},
			});

			if (error) {
				localFailure = removalFailureFor(error, member);
				localRemoval = { ...removal, kind: "ready" };

				return;
			}

			replace(members.filter((row) => row.accountId !== member.accountId));
			localNotice = {
				kind: "removed",
				name: memberName(member),
				reassigned: target ? memberName(target) : null,
			};
			announcement = `${memberName(member)} was removed.`;
			await closeRemoval();
		} catch {
			localFailure = { kind: "unavailable" };
			localRemoval = { ...removal, kind: "ready" };
		}
	}

	async function closeRemoval() {
		const next = new URL(page.url);
		next.searchParams.delete("remove");

		await goto(next, { replaceState: true, noScroll: true });
	}

	function removeHref(member: Membership): string {
		const next = new URL(page.url);
		next.searchParams.set("remove", member.accountId);

		return `${next.pathname}${next.search}`;
	}

	function rowLocked(member: Membership): boolean {
		return busy || isDirectoryManaged(member) || member.accountId === viewerId;
	}
</script>

<svelte:head><title>Members · {workspace.name} · Norn</title></svelte:head>

<div class="flex min-h-0 flex-1 flex-col">
	<div class="flex-none border-b border-line-default">
		<div class="flex h-11 items-center gap-2 pr-3 pl-4">
			<UserRound class="size-icon-toolbar shrink-0 text-muted-foreground" aria-hidden="true" />
			<h1 class="text-md font-medium tracking-snug whitespace-nowrap text-ink-900">Members</h1>
			<div class="flex-1"></div>
			<Button size="sm" href="/invite-teammates?workspace={slug}">Invite people</Button>
		</div>
	</div>

	<div class="flex-1 overflow-auto">
		<div
			class="mx-auto flex w-full max-w-140 flex-col gap-6 px-4 py-6 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))]"
		>
			<p class="sr-only" aria-live="polite" aria-atomic="true">{announcement}</p>

			{#if notice}
				<Alert.Root variant="success">
					<CircleCheck aria-hidden="true" />
					{#if notice.kind === "role_changed"}
						<Alert.Title>{notice.name} is now {roleLabels[notice.role]}</Alert.Title>
						<Alert.Description>They see the change the next time they load a page.</Alert.Description>
					{:else}
						<Alert.Title>{notice.name} was removed from {workspace.name}</Alert.Title>
						<Alert.Description>
							{notice.reassigned
								? `Their work is now assigned to ${notice.reassigned}.`
								: "Nothing was reassigned."}
						</Alert.Description>
					{/if}
				</Alert.Root>
			{/if}

			{#if failure}
				<Alert.Root variant="destructive">
					<CircleX aria-hidden="true" />
					<Alert.Title>{membershipFailureTitle(failure)}</Alert.Title>
					<Alert.Description>{membershipFailureMessage(failure)}</Alert.Description>
				</Alert.Root>
			{/if}

			<section class="flex flex-col gap-4">
				<div class="flex flex-col gap-1">
					<h2 class="text-md font-medium tracking-snug text-ink-900">Roles</h2>
					<dl class="flex flex-col gap-0.5">
						{#each membershipRoles as role (role)}
							<div class="flex flex-wrap gap-x-2 text-sm leading-normal">
								<dt class="font-medium text-ink-900">{roleLabels[role]}</dt>
								<dd class="min-w-0 flex-1 text-muted-foreground text-pretty">{roleNotes[role]}</dd>
							</div>
						{/each}
					</dl>
					{#if directoryInView}
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							Accounts marked Directory are provisioned by your identity provider. Change their role
							there.
						</p>
					{/if}
				</div>

				<div class="flex flex-col gap-1.5" role="search">
					<label for="member-search" class="text-sm font-medium text-ink-900">Search members</label>
					<Input
						id="member-search"
						type="search"
						enterkeyhint="search"
						autocapitalize="none"
						spellcheck="false"
						placeholder="Name or email"
						disabled={busy}
						value={draft}
						oninput={(event) => type(event.currentTarget.value)}
						onkeydown={flush}
					/>
				</div>

				{#if listing.kind === "loading"}
					<ul class="flex flex-col gap-px" aria-busy="true">
						{#each [0, 1, 2] as row (row)}
							<li class="h-14 animate-pulse rounded-md bg-paper-2"></li>
						{/each}
					</ul>
				{:else if listing.kind === "unavailable"}
					<Alert.Root variant="destructive">
						<CircleX aria-hidden="true" />
						<Alert.Title>We could not load the members</Alert.Title>
						<Alert.Description>Nothing changed. Wait a moment and try again.</Alert.Description>
					</Alert.Root>
				{:else if listing.kind === "empty"}
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						This workspace has no members yet.
					</p>
				{:else if listing.kind === "no_matches"}
					<div class="flex flex-col gap-2">
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							Nobody in {workspace.name} matches “{listing.query}”.
						</p>
						<div>
							<Button variant="secondary" size="sm" href={workspacePath(slug, "/settings/members")}>
								Clear search
							</Button>
						</div>
					</div>
				{:else}
					<ul class="flex flex-col rounded-lg border border-line-default">
						{#each members as member (member.accountId)}
							<li class="border-b border-line-subtle last:border-b-0">
								{#if removalId === member.accountId && removal.kind !== "closed"}
									<div class="flex flex-col gap-3 bg-paper-2 px-3 py-3" role="group">
										{#if removal.kind === "loading"}
											<p class="text-sm text-muted-foreground">Checking what this would affect…</p>
										{:else if removal.kind === "unavailable"}
											<p class="text-sm text-destructive">
												We could not check what removing {memberName(member)} would affect.
											</p>
											<div>
												<Button variant="ghost" size="sm" onclick={closeRemoval}>Cancel</Button>
											</div>
										{:else if removalMember}
											<h3 class="text-md font-medium tracking-snug text-ink-900">
												Remove {memberName(removalMember)} from {workspace.name}?
											</h3>

											{#if "soleAdmin" in removal && removal.soleAdmin}
												<p class="text-sm leading-normal text-destructive text-pretty">
													{memberName(removalMember)} is the only admin. A workspace always needs one,
													so this cannot go ahead until someone else is made an admin.
												</p>
											{:else}
												<p class="text-sm leading-normal text-muted-foreground text-pretty">
													They lose access immediately. You can invite them back at any time.
												</p>
											{/if}

											{#if "teams" in removal && removal.teams.length > 0}
												<div class="flex flex-col gap-1.5">
													<p class="text-sm leading-normal text-muted-foreground text-pretty">
														They are on {removal.teams.length} team{removal.teams.length === 1
															? ""
															: "s"}. Removing them from {workspace.name} takes them out of all of
														them.
													</p>
													<ul class="flex flex-wrap gap-x-3 gap-y-1">
														{#each removal.teams as team (team.id)}
															<li class="flex items-center gap-1.5">
																<TeamKey key={team.key} />
																<span class="text-sm text-muted-foreground">{team.name}</span>
															</li>
														{/each}
													</ul>
												</div>
											{/if}

											{#if members.length > 1}
												<div class="flex flex-col gap-1.5">
													<span class="text-sm font-medium text-ink-900">Give their work to</span>
													<Select.Root
														type="single"
														value={reassignTo}
														disabled={busy}
														onValueChange={(value) => (reassignTo = value)}
													>
														<Select.Trigger aria-label="Reassign the work of {memberName(removalMember)}">
															{members.find((row) => row.accountId === reassignTo)
																? memberName(members.find((row) => row.accountId === reassignTo)!)
																: "Leave it unassigned"}
														</Select.Trigger>
														<Select.Content>
															{#each members.filter((row) => row.accountId !== removalMember.accountId) as candidate (candidate.accountId)}
																<Select.Item
																	value={candidate.accountId}
																	label={memberName(candidate)}
																>
																	{memberName(candidate)}
																</Select.Item>
															{/each}
														</Select.Content>
													</Select.Root>
												</div>
											{/if}

											<div class="flex flex-wrap gap-2">
												<Button variant="destructive" disabled={busy} onclick={confirmRemoval}>
													{removal.kind === "removing"
														? "Removing"
														: reassignTo
															? "Remove and reassign"
															: "Remove without reassigning"}
												</Button>
												<Button variant="ghost" disabled={busy} onclick={closeRemoval}>Cancel</Button>
											</div>
										{/if}
									</div>
								{:else}
									<div class="flex flex-wrap items-center gap-x-2 gap-y-1 px-3 py-2">
										<Avatar.Root size="sm">
											<Avatar.Fallback>{initialsOf(memberName(member))}</Avatar.Fallback>
										</Avatar.Root>

										<span class="min-w-0 flex-[1_1_120px] truncate text-md text-ink-900">
											{memberName(member)}
										</span>

										{#if member.kind === "agent"}
											<Tag name="Agent" />
										{/if}

										{#if member.accountId === viewerId}
											<Tag name="You" />
										{:else if isDirectoryManaged(member)}
											<Tag name="Directory" />
										{/if}

										<span class="ml-auto flex shrink-0 items-center gap-1.5">
											<span class="w-[104px]">
												<Select.Root
													type="single"
													value={member.role}
													disabled={rowLocked(member)}
													onValueChange={(value) => changeRole(member, value as MembershipRole)}
												>
													<Select.Trigger size="sm" aria-label="Role for {memberName(member)}">
														{roleLabels[member.role]}
													</Select.Trigger>
													<Select.Content>
														{#each membershipRoles as role (role)}
															<Select.Item value={role} label={roleLabels[role]}>
																{roleLabels[role]}
															</Select.Item>
														{/each}
													</Select.Content>
												</Select.Root>
											</span>
											{#if member.accountId !== viewerId}
												<Button
													variant="ghost"
													size="icon-sm"
													disabled={busy || isDirectoryManaged(member)}
													aria-label="Remove {memberName(member)} from {workspace.name}"
													href={removeHref(member)}
												>
													<X aria-hidden="true" />
												</Button>
											{/if}
										</span>

										<p
											class="flex min-w-0 flex-[1_1_100%] flex-wrap items-center gap-x-2 text-sm text-muted-foreground"
										>
											{#if member.email}
												<span class="truncate font-mono text-xs">{member.email}</span>
												<span aria-hidden="true">·</span>
											{/if}
											<span>
												<time datetime={member.joinedAt}>{joinedOn(member.joinedAt, data.workspace.timezone)}</time>
											</span>
											<span aria-hidden="true">·</span>
											<span>{lastActive(member.lastActiveAt, data.now, data.workspace.timezone)}</span>
											{#if linked.has(member.accountId)}
												<span aria-hidden="true">·</span>
												<span>Linked to your provider</span>
											{:else if authMethodLabel(member.lastAuthMethod)}
												<span aria-hidden="true">·</span>
												<span>{authMethodLabel(member.lastAuthMethod)}</span>
											{/if}
										</p>
									</div>
								{/if}
							</li>
						{/each}
					</ul>

					{#if removalId && removal.kind === "not_in_view"}
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							That person is not in this list. Search for them by name or email, then remove them
							from their row.
						</p>
					{/if}

					{#if hasMore(listing)}
						<div class="flex flex-col gap-1.5">
							<div>
								<Button
									variant="secondary"
									disabled={busy || paging.kind === "loading"}
									onclick={loadMore}
								>
									{paging.kind === "loading" ? "Loading" : "Load more people"}
								</Button>
							</div>
							{#if paging.kind === "unavailable"}
								<p class="text-sm text-destructive">
									We could not load any more people. Wait a moment and try again.
								</p>
							{/if}
						</div>
					{:else}
						<p class="text-sm leading-normal text-muted-foreground">
							{committed ? `That’s everyone matching “${committed}”.` : "That’s everyone."}
						</p>
					{/if}
				{/if}
			</section>

			{#if tokens}
			<section class="flex flex-col gap-3">
				<div class="flex flex-col gap-1">
					<h2 class="text-md font-medium tracking-snug text-ink-900">
						API tokens reaching {workspace.name}
					</h2>
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						A token acts for the person who minted it and carries only what they may do. Revoking
						one here stops it everywhere at once.
					</p>
				</div>

				{#if tokens.length === 0}
					<p class="text-sm text-muted-foreground">
						No tokens reach this workspace.
					</p>
				{:else}
					<ul class="rounded-lg border border-line-subtle bg-paper-0">
						{#each tokens as owned (owned.token.id)}
							<li
								class="flex flex-col gap-2 border-b border-line-subtle p-3 last:border-b-0 sm:flex-row sm:items-start sm:justify-between"
							>
								<div class="flex min-w-0 flex-col gap-1.5">
									<div class="flex items-center gap-2">
										<Plug class="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
										<span class="truncate text-sm text-ink-900">{owned.token.name}</span>
									</div>
									<p class="text-xs text-muted-foreground">
										Acts for {owned.ownerName} &lt;{owned.ownerEmail}&gt;
									</p>
									<div class="flex flex-wrap gap-1">
										{#each owned.token.scopes as scope (scope)}
											<Tag name={scope} />
										{/each}
									</div>
								</div>
								<div class="flex-none">
									<Button
										variant="secondary"
										size="sm"
										disabled={busy || revokingToken === owned.token.id}
										onclick={() => revokeToken(owned.token.id)}
									>
										{revokingToken === owned.token.id ? "Revoking…" : "Revoke"}
									</Button>
								</div>
							</li>
						{/each}
					</ul>
				{/if}
			</section>
			{/if}
		</div>
	</div>
</div>

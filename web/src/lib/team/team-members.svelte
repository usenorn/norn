<script lang="ts">
	import { invalidate } from "$app/navigation";
	import { page } from "$app/state";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import X from "@lucide/svelte/icons/x";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Avatar from "$lib/components/ui/avatar/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { api } from "$lib/api";
	import { keys } from "$lib/api/keys";
	import {
		memberFailureMessage,
		membersOf,
		rosterFor,
		type MemberFailure,
		type TeamRoster,
	} from "$lib/team/members";
	import { initialsOf } from "$lib/team/members";
	import { memberName, searchDebounceMs, type Membership } from "$lib/workspace/members";

	const candidateLimit = 8;

	let {
		workspace,
		team,
		roster,
		failure = null,
		readOnly = false,
		archived = false,
		busy = false,
	}: {
		workspace: { id: string; name: string; slug: string };
		team: { id: string; name: string };
		roster: TeamRoster;
		failure?: MemberFailure | null;
		readOnly?: boolean;
		archived?: boolean;
		busy?: boolean;
	} = $props();

	let changed = $state.raw<TeamRoster | null>(null);
	let refused = $state.raw<MemberFailure | null>(null);
	let candidateQuery = $state("");
	let candidates = $state.raw<Membership[]>([]);
	let searching = $state(false);
	let adding = $state("");
	let removing = $state("");
	let candidateDebounce: ReturnType<typeof setTimeout> | undefined;

	const standing = $derived(changed ?? roster);
	const members = $derived(membersOf(standing));
	const shown = $derived(refused ?? failure);
	const working = $derived(busy || adding !== "" || removing !== "");
	const locked = $derived(working || archived || readOnly);

	$effect(() => () => clearTimeout(candidateDebounce));

	async function findCandidates(query: string) {
		if (!query) {
			candidates = [];

			return;
		}

		searching = true;

		try {
			const { data: found } = await api.GET("/workspaces/{workspaceId}/members", {
				params: { path: { workspaceId: workspace.id }, query: { query, limit: candidateLimit } },
			});

			candidates = (found?.members ?? []).filter(
				(candidate) => !members.some((member) => member.accountId === candidate.accountId)
			);
		} catch {
			candidates = [];
		} finally {
			searching = false;
		}
	}

	function searchCandidates(value: string) {
		candidateQuery = value;
		clearTimeout(candidateDebounce);
		candidateDebounce = setTimeout(() => void findCandidates(value), searchDebounceMs);
	}

	async function addMember(accountId: string) {
		adding = accountId;
		refused = null;

		try {
			const { data: added, error } = await api.POST(
				"/workspaces/{workspaceId}/teams/{teamId}/members",
				{
					params: { path: { workspaceId: workspace.id, teamId: team.id } },
					body: { accountId },
				}
			);

			if (added) {
				changed = { kind: "added", members: [...members, added], member: added };
				candidates = candidates.filter((candidate) => candidate.accountId !== accountId);
				await invalidate(keys.page(page.route.id));

				return;
			}

			if (error?.status === 403) {
				refused = { kind: "forbidden" };

				return;
			}

			if (error && "code" in error && error.code === "team_member_exists") {
				refused = { kind: "already_member" };

				return;
			}

			refused = error?.status === 404 ? { kind: "not_in_workspace" } : { kind: "unavailable" };
		} catch {
			refused = { kind: "unavailable" };
		} finally {
			adding = "";
		}
	}

	async function removeMember(accountId: string) {
		removing = accountId;
		refused = null;

		try {
			const { error } = await api.DELETE(
				"/workspaces/{workspaceId}/teams/{teamId}/members/{accountId}",
				{ params: { path: { workspaceId: workspace.id, teamId: team.id, accountId } } }
			);

			if (error) {
				refused = error.status === 403 ? { kind: "forbidden" } : { kind: "unavailable" };

				return;
			}

			changed = rosterFor(members.filter((member) => member.accountId !== accountId));
			await invalidate(keys.page(page.route.id));
		} catch {
			refused = { kind: "unavailable" };
		} finally {
			removing = "";
		}
	}
</script>

<div class="flex flex-col gap-4">
	{#if shown}
		<Alert.Root variant="destructive">
			<CircleX aria-hidden="true" />
			<Alert.Title>That did not work</Alert.Title>
			<Alert.Description>{memberFailureMessage(shown)}</Alert.Description>
		</Alert.Root>
	{/if}

	{#if standing.kind === "loading"}
		<div class="h-20 animate-breathe rounded-lg bg-paper-2" aria-busy="true"></div>
	{:else if standing.kind === "unavailable"}
		<p class="text-sm leading-normal text-muted-foreground">
			We could not load who is on this team.
		</p>
	{:else if members.length === 0}
		<p class="text-sm leading-normal text-muted-foreground">Nobody is on this team yet.</p>
	{:else}
		<ul class="flex flex-col rounded-lg border border-line-default" aria-live="polite">
			{#each members as member (member.accountId)}
				<li
					class="flex flex-wrap items-center gap-2 border-b border-line-subtle px-3 py-2 last:border-b-0"
				>
					<Avatar.Root size="sm">
						<Avatar.Fallback>{initialsOf(member.displayName)}</Avatar.Fallback>
					</Avatar.Root>
					<span class="min-w-0 flex-[1_1_120px] truncate text-md text-ink-900">
						{member.displayName}
					</span>
					<span class="min-w-0 truncate text-sm text-muted-foreground">{member.email}</span>
					{#if !archived && !readOnly}
						<Button
							variant="ghost"
							size="icon-sm"
							disabled={locked}
							aria-label="Remove {member.displayName} from {team.name}"
							onclick={() => void removeMember(member.accountId)}
						>
							<X aria-hidden="true" />
						</Button>
					{/if}
				</li>
			{/each}
		</ul>
	{/if}

	{#if !archived && !readOnly}
		<div class="flex flex-col gap-2" role="search">
			<label for="team-member-search" class="text-sm font-medium text-ink-900">Add someone</label>
			<Input
				id="team-member-search"
				type="search"
				enterkeyhint="search"
				autocapitalize="none"
				spellcheck="false"
				placeholder="Search {workspace.name} by name or email"
				disabled={working}
				value={candidateQuery}
				oninput={(event) => searchCandidates(event.currentTarget.value)}
			/>

			{#if candidateQuery && candidates.length === 0 && !searching}
				<p class="text-sm leading-normal text-muted-foreground text-pretty">
					Nobody in {workspace.name} matches “{candidateQuery}”.
					<a
						href="/invite-teammates?workspace={workspace.slug}"
						class="text-link underline-offset-2 hover:text-link-hover hover:underline"
					>
						Invite them to {workspace.name}
					</a>
					first.
				</p>
			{:else if candidates.length > 0}
				<ul class="flex flex-col rounded-lg border border-line-default">
					{#each candidates as candidate (candidate.accountId)}
						<li class="border-b border-line-subtle last:border-b-0">
							<Button
								variant="ghost"
								class="h-auto w-full justify-start gap-2 rounded-none px-3 py-2"
								disabled={working}
								onclick={() => void addMember(candidate.accountId)}
							>
								<Avatar.Root size="sm">
									<Avatar.Fallback>{initialsOf(memberName(candidate))}</Avatar.Fallback>
								</Avatar.Root>
								<span class="min-w-0 flex-1 truncate text-left text-md text-ink-900">
									{memberName(candidate)}
								</span>
								<span class="shrink-0 text-sm text-muted-foreground">
									{adding === candidate.accountId ? "Adding" : "Add"}
								</span>
							</Button>
						</li>
					{/each}
				</ul>
			{/if}
		</div>
	{/if}
</div>

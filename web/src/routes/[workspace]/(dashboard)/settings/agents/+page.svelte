<script lang="ts">
	import { goto, invalidate } from "$app/navigation";
	import { page } from "$app/state";
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import Bot from "@lucide/svelte/icons/bot";
	import MoreHorizontal from "@lucide/svelte/icons/more-horizontal";
	import Plus from "@lucide/svelte/icons/plus";
	import RotateCw from "@lucide/svelte/icons/rotate-cw";
	import Search from "@lucide/svelte/icons/search";
	import ShieldCheck from "@lucide/svelte/icons/shield-check";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import UserX from "@lucide/svelte/icons/user-x";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as AlertDialog from "$lib/components/ui/alert-dialog/index.js";
	import * as DropdownMenu from "$lib/components/ui/dropdown-menu/index.js";
	import * as Empty from "$lib/components/ui/empty/index.js";
	import * as Select from "$lib/components/ui/select/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { Skeleton } from "$lib/components/ui/skeleton/index.js";
	import SettingsPage from "$lib/settings/settings-page.svelte";
	import AgentIdentity from "$lib/agents/agent-identity.svelte";
	import CredentialDialog from "$lib/agents/credential-dialog.svelte";
	import RegisterAgentDialog from "$lib/agents/register-agent-dialog.svelte";
	import Tag from "$lib/components/norn/tag.svelte";
	import { api } from "$lib/api";
	import { keys } from "$lib/api/keys";
	import { listCursor } from "$lib/shortcuts/list-cursor.svelte";
	import { bindShortcut, holdShortcuts } from "$lib/shortcuts/registry.svelte";
	import ShortcutBar from "$lib/shortcuts/shortcut-bar.svelte";
	import { onDate } from "$lib/time";
	import {
		agentPath,
		approvalsPath,
		failureMessage,
		type Agent,
		type AgentFailure,
		type AgentListing,
		type WorkspaceAgent,
	} from "$lib/agents/agents";
	import { runnersPath } from "$lib/runners/runners";
	import { agentsPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	type StatusFilter = "all" | Agent["status"];
	type AgentConfirmation = {
		kind: "rotate" | "disable" | "enable";
		owned: WorkspaceAgent;
	};
	type AgentMutation =
		| { kind: "idle" }
		| { kind: "rotating"; agentId: string }
		| { kind: "disabling"; agentId: string }
		| { kind: "enabling"; agentId: string };

	const statusLabels: Record<StatusFilter, string> = {
		all: "All agents",
		active: "Active",
		disabled: "Disabled",
	};
	const statusFilters = ["all", "active", "disabled"] as const satisfies readonly StatusFilter[];

	let { data, form: submitted }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV ? agentsPreviewStates[page.url.searchParams.get("state") ?? ""] : undefined
	);
	const inlineRegistration = $derived(page.url.searchParams.get("register") === "1");
	const registerHref = $derived.by(() => {
		const search = new URLSearchParams(page.url.searchParams);
		search.set("register", "1");

		return `${page.url.pathname}?${search}`;
	});

	let query = $state("");
	let status = $state<StatusFilter>("all");
	let registerOpen = $state(false);
	let confirmation = $state.raw<AgentConfirmation | null>(null);
	let mutation = $state.raw<AgentMutation>({ kind: "idle" });
	let localFailure = $state<AgentFailure | null>(null);
	let issuedAgent = $state.raw<Agent | null>(null);
	let issuedValue = $state("");
	let issuedByRotation = $state(false);
	let announced = $state("");
	let deliveredPreview = $state("");

	const workspace = $derived(data.workspace);
	const listing = $derived<AgentListing>(preview?.listing ?? data.listing);
	const teams = $derived(preview?.teams ?? data.teams);
	const allAgents = $derived(
		listing.kind === "ready" || listing.kind === "registered" ? listing.agents : []
	);
	const normalizedQuery = $derived(query.trim().toLowerCase());
	const visibleAgents = $derived(
		allAgents.filter((owned) => {
			if (status !== "all" && owned.agent.status !== status) return false;
			if (!normalizedQuery) return true;

			return [owned.agent.name, owned.ownerName, owned.ownerEmail].some((value) =>
				value.toLowerCase().includes(normalizedQuery)
			);
		})
	);
	const showRegister = $derived(listing.kind !== "forbidden" && listing.kind !== "unavailable");
	const busy = $derived(preview?.busy || mutation.kind !== "idle");
	const failure = $derived(preview?.failure ?? localFailure);
	const activeCount = $derived(allAgents.filter((owned) => owned.agent.status === "active").length);

	const cursor = listCursor(() => ({
		rows: visibleAgents,
		keyOf: (owned) => owned.agent.id,
		open: (owned) => void goto(agentPath(workspace.slug, owned.agent.id)),
	}));

	bindShortcut("agent-register", () => {
		if (showRegister && !busy && !inlineRegistration) registerOpen = true;
	});
	holdShortcuts(
		() => inlineRegistration || registerOpen || confirmation !== null || issuedAgent !== null
	);

	$effect(() => {
		const registered = listing.kind === "registered" ? listing : null;

		if (!registered || registered.value === deliveredPreview) return;

		deliveredPreview = registered.value;
		issuedAgent = registered.agent;
		issuedValue = registered.value;
		issuedByRotation = false;
	});

	function chooseStatus(value: string) {
		if (value === "all" || value === "active" || value === "disabled") status = value;
	}

	function openRegister(event: MouseEvent) {
		event.preventDefault();

		if (!busy) registerOpen = true;
	}

	function mutationFailure(problem: unknown): AgentFailure {
		if (!problem || typeof problem !== "object") return { kind: "unavailable" };
		if ("status" in problem && problem.status === 403) return { kind: "forbidden" };
		if (!("code" in problem)) return { kind: "unavailable" };

		switch (problem.code) {
			case "agent_owner_invalid":
				return { kind: "lifecycle_owner_invalid" };
			case "agent_disabled":
				return { kind: "disabled" };
			case "agent_active":
				return { kind: "active" };
			case "agent_authority_missing":
				return { kind: "authority_missing" };
			default:
				return { kind: "unavailable" };
		}
	}

	async function refresh(agentId: string) {
		await Promise.all([
			invalidate(keys.agents(workspace.id)),
			invalidate(keys.agent(workspace.id, agentId)),
		]);
	}

	function refreshAfterMutation(agentId: string) {
		void refresh(agentId).catch(() => undefined);
	}

	async function rotate(owned: WorkspaceAgent) {
		mutation = { kind: "rotating", agentId: owned.agent.id };
		localFailure = null;

		try {
			const { data: issued, error } = await api.POST(
				"/workspaces/{workspaceId}/agents/{agentId}/credential",
				{ params: { path: { workspaceId: workspace.id, agentId: owned.agent.id } } }
			);

			if (error || !issued) {
				localFailure = mutationFailure(error);

				return;
			}

			confirmation = null;
			issuedAgent = issued.agent;
			issuedValue = issued.value;
			issuedByRotation = true;
			announced = `${issued.agent.name} has a new credential.`;
			refreshAfterMutation(owned.agent.id);
		} catch {
			localFailure = { kind: "unavailable" };
		} finally {
			mutation = { kind: "idle" };
			confirmation = null;
		}
	}

	async function disable(owned: WorkspaceAgent) {
		mutation = { kind: "disabling", agentId: owned.agent.id };
		localFailure = null;

		try {
			const { error } = await api.DELETE("/workspaces/{workspaceId}/agents/{agentId}", {
				params: { path: { workspaceId: workspace.id, agentId: owned.agent.id } },
			});

			if (error) {
				localFailure = mutationFailure(error);

				return;
			}

			confirmation = null;
			announced = `${owned.agent.name} is disabled.`;
			refreshAfterMutation(owned.agent.id);
		} catch {
			localFailure = { kind: "unavailable" };
		} finally {
			mutation = { kind: "idle" };
			confirmation = null;
		}
	}

	async function enable(owned: WorkspaceAgent) {
		mutation = { kind: "enabling", agentId: owned.agent.id };
		localFailure = null;

		try {
			const { data: issued, error } = await api.POST(
				"/workspaces/{workspaceId}/agents/{agentId}/enable",
				{ params: { path: { workspaceId: workspace.id, agentId: owned.agent.id } } }
			);

			if (error || !issued) {
				localFailure = mutationFailure(error);

				return;
			}

			confirmation = null;
			issuedAgent = issued.agent;
			issuedValue = issued.value;
			issuedByRotation = true;
			announced = `${issued.agent.name} is active with a new credential.`;
			refreshAfterMutation(owned.agent.id);
		} catch {
			localFailure = { kind: "unavailable" };
		} finally {
			mutation = { kind: "idle" };
			confirmation = null;
		}
	}

	async function registered(agent: Agent, value: string) {
		issuedAgent = agent;
		issuedValue = value;
		issuedByRotation = false;
		announced = `${agent.name} is registered.`;
		await invalidate(keys.agents(workspace.id));
	}

	function confirm() {
		const pending = confirmation;

		if (!pending) return;
		if (pending.kind === "rotate") void rotate(pending.owned);
		if (pending.kind === "disable") void disable(pending.owned);
		if (pending.kind === "enable") void enable(pending.owned);
	}

	function formatted(instant: string): string {
		return onDate(instant, workspace.timezone);
	}

	function closeCredential() {
		issuedAgent = null;
		issuedValue = "";
	}
</script>

{#snippet registration(inline: boolean)}
	<RegisterAgentDialog
		open={!inline && registerOpen}
		workspaceId={workspace.id}
		workspaceName={workspace.name}
		origin={page.url.origin}
		{teams}
		initial={submitted?.form ?? data.form}
		{inline}
		closeHref={page.url.pathname}
		disabled={busy}
		onclose={() => (registerOpen = false)}
		onissued={registered}
	/>
{/snippet}

<svelte:head><title>Agents · {workspace.name} · Norn</title></svelte:head>

<SettingsPage
	title="Agents"
	description="Bound identities for MCP clients and delegated work."
	Icon={Bot}
	meta={listing.kind === "loading" ? "loading" : `${activeCount} active / ${allAgents.length} total`}
>
	{#snippet actions()}
		<div class="flex items-center gap-3">
			<a
				href={runnersPath(workspace.slug)}
				class="text-xs text-muted-foreground motion-control hover:text-ink-900"
			>
				Runners
			</a>
			<a
				href={approvalsPath(workspace.slug)}
				class="text-xs text-muted-foreground motion-control hover:text-ink-900"
			>
				Approvals
			</a>
			{#if showRegister}
				<Button href={registerHref} size="sm" disabled={busy} onclick={openRegister}>
					<Plus aria-hidden="true" />
					Register
				</Button>
			{/if}
		</div>
	{/snippet}

			<p class="sr-only" aria-live="polite" aria-atomic="true">{announced}</p>

			{#if inlineRegistration && showRegister}
				{@render registration(true)}
			{:else}
				{#if failure}
					<Alert.Root variant="destructive">
						<TriangleAlert aria-hidden="true" />
						<Alert.Title>Agent action failed</Alert.Title>
						<Alert.Description>{failureMessage(failure)}</Alert.Description>
					</Alert.Root>
				{/if}

				{#if listing.kind === "forbidden"}
					<Alert.Root variant="muted">
						<CircleAlert aria-hidden="true" />
						<Alert.Title>You may not manage agents here</Alert.Title>
						<Alert.Description>Ask an administrator of {workspace.name}.</Alert.Description>
					</Alert.Root>
				{:else if listing.kind === "authority_missing"}
					<Alert.Root variant="warning">
						<CircleAlert aria-hidden="true" />
						<Alert.Title>An agent has no authority to restore</Alert.Title>
						<Alert.Description>
							Register a replacement agent with the authority it needs.
						</Alert.Description>
						<Alert.Action>
							<Button href={registerHref} variant="secondary" size="sm" onclick={openRegister}>
								Register replacement
							</Button>
						</Alert.Action>
					</Alert.Root>
				{:else if listing.kind === "unavailable"}
					<Alert.Root variant="destructive">
						<TriangleAlert aria-hidden="true" />
						<Alert.Title>Could not load the agents</Alert.Title>
						<Alert.Description>Check your connection and reload.</Alert.Description>
					</Alert.Root>
				{:else}
					<section class="flex flex-col gap-3" aria-labelledby="agent-roster-heading">
						<div class="flex flex-col gap-1">
							<h2 id="agent-roster-heading" class="text-md font-medium tracking-snug text-ink-900">
								Command roster
							</h2>
							<p class="text-sm leading-normal text-muted-foreground">
								Review who each agent acts for, its reach, and whether its credential is active.
							</p>
						</div>

						<div class="grid gap-3 sm:grid-cols-[minmax(0,1fr)_12rem]" role="search">
							<div class="flex min-w-0 flex-col gap-1.5">
								<label for="agent-search" class="flex items-center gap-1.5 text-sm font-medium text-ink-900">
									<Search class="size-3.5 text-muted-foreground" aria-hidden="true" />
									Search agents
								</label>
								<Input
									id="agent-search"
									type="search"
									bind:value={query}
									placeholder="Agent or owner"
									autocapitalize="none"
									spellcheck="false"
									disabled={busy}
								/>
							</div>
							<div class="flex min-w-0 flex-col gap-1.5">
								<label for="agent-status" class="text-sm font-medium text-ink-900">Status</label>
								<Select.Root
									type="single"
									value={status}
									onValueChange={chooseStatus}
									disabled={busy}
								>
									<Select.Trigger id="agent-status">{statusLabels[status]}</Select.Trigger>
									<Select.Content>
										{#each statusFilters as option (option)}
											<Select.Item value={option}>{statusLabels[option]}</Select.Item>
										{/each}
									</Select.Content>
								</Select.Root>
							</div>
						</div>

						{#if listing.kind === "loading"}
							<ul class="flex flex-col gap-px" aria-busy="true" aria-label="Loading agents">
								{#each [0, 1, 2, 3] as row (row)}
									<li class="flex min-h-15 items-center gap-3 border-b border-line-subtle px-3 py-2">
										<Skeleton class="size-8 shrink-0" />
										<div class="flex flex-1 flex-col gap-2">
											<Skeleton class="h-3 w-36" />
											<Skeleton class="h-2.5 w-52 max-w-full" />
										</div>
									</li>
								{/each}
							</ul>
						{:else if allAgents.length === 0}
							<Empty.Root>
								<Empty.Media variant="icon"><ShieldCheck aria-hidden="true" /></Empty.Media>
								<Empty.Header>
									<Empty.Title>No agents registered</Empty.Title>
									<Empty.Description>
										Register an agent when a workflow needs its own identity and bounded authority.
									</Empty.Description>
								</Empty.Header>
								{#if showRegister}
									<Empty.Content>
										<Button href={registerHref} size="sm" onclick={openRegister}>
											<Plus aria-hidden="true" />
											Register agent
										</Button>
									</Empty.Content>
								{/if}
							</Empty.Root>
						{:else if visibleAgents.length === 0}
							<Empty.Root>
								<Empty.Media variant="icon"><Search aria-hidden="true" /></Empty.Media>
								<Empty.Header>
									<Empty.Title>No matching agents</Empty.Title>
									<Empty.Description>Try another name, owner, or status.</Empty.Description>
								</Empty.Header>
								<Empty.Content>
									<Button
										variant="secondary"
										size="sm"
										onclick={() => {
											query = "";
											status = "all";
										}}
									>
										Clear filters
									</Button>
								</Empty.Content>
							</Empty.Root>
						{:else}
							<ul class="overflow-hidden rounded-lg border border-line-default bg-paper-0">
								{#each visibleAgents as owned (owned.agent.id)}
									<li
										{...cursor.props(owned)}
										class="grid min-w-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-3 border-b border-line-subtle px-3 py-2 last:border-b-0 data-[cursor=true]:rule-inset data-[cursor=true]:bg-accent"
									>
										<div class="flex min-w-0 flex-col gap-1.5">
											<AgentIdentity
												agent={owned.agent}
												href={agentPath(workspace.slug, owned.agent.id)}
												description={`Acts for ${owned.ownerName || owned.ownerEmail || "a former member"}`}
											/>
											<div class="flex flex-wrap items-center gap-x-3 gap-y-1 pl-11">
												<Tag name={owned.agent.status === "active" ? "Active" : "Disabled"} />
												<span class="font-mono text-2xs text-muted-foreground">
													{owned.authority.scopes.length} permissions
												</span>
												<span class="font-mono text-2xs text-muted-foreground">
													{owned.authority.allTeams
														? "All reachable teams"
														: `${owned.authority.teamIds.length} teams`}
												</span>
												<span class="font-mono text-2xs text-muted-foreground">
													{owned.agent.actionLimit}/min
												</span>
												<span class="text-2xs text-muted-foreground">
													Registered {formatted(owned.agent.createdAt)}
												</span>
											</div>
										</div>

										<DropdownMenu.Root>
											<DropdownMenu.Trigger disabled={busy}>
												{#snippet child({ props })}
													<Button
														{...props}
														variant="ghost"
														size="icon-sm"
														aria-label={`Actions for ${owned.agent.name}`}
													>
														<MoreHorizontal aria-hidden="true" />
													</Button>
												{/snippet}
											</DropdownMenu.Trigger>
											<DropdownMenu.Content align="end">
												{#if owned.agent.status === "active"}
													<DropdownMenu.Item
														onSelect={() => (confirmation = { kind: "rotate", owned })}
													>
														<RotateCw aria-hidden="true" />
														Rotate credential
													</DropdownMenu.Item>
													<DropdownMenu.Item
														variant="destructive"
														onSelect={() => (confirmation = { kind: "disable", owned })}
													>
														<UserX aria-hidden="true" />
														Disable agent
													</DropdownMenu.Item>
												{:else}
													<DropdownMenu.Item
														onSelect={() => (confirmation = { kind: "enable", owned })}
													>
														<ShieldCheck aria-hidden="true" />
														Enable agent
													</DropdownMenu.Item>
												{/if}
											</DropdownMenu.Content>
										</DropdownMenu.Root>
									</li>
								{/each}
							</ul>
						{/if}
					</section>
				{/if}
			{/if}
</SettingsPage>

<ShortcutBar ids={["cursor-down", "cursor-open", "agent-register", "search", "help"]} />

{#if showRegister && !inlineRegistration}
	{@render registration(false)}
{/if}

<CredentialDialog
	open={issuedAgent !== null}
	agent={issuedAgent}
	value={issuedValue}
	origin={page.url.origin}
	workspaceName={workspace.name}
	rotated={issuedByRotation}
	onclose={closeCredential}
/>

<AlertDialog.Root
	open={confirmation !== null}
	onOpenChange={(open) => {
		if (!open && mutation.kind === "idle") confirmation = null;
	}}
>
	<AlertDialog.Content size="sm">
		<AlertDialog.Header>
			<AlertDialog.Title>
				{#if confirmation?.kind === "rotate"}
					Rotate {confirmation.owned.agent.name}'s credential?
				{:else if confirmation?.kind === "disable"}
					Disable {confirmation.owned.agent.name}?
				{:else if confirmation?.kind === "enable"}
					Enable {confirmation.owned.agent.name}?
				{/if}
			</AlertDialog.Title>
			<AlertDialog.Description>
				{#if confirmation?.kind === "rotate"}
					The current credential stops working immediately. The replacement is shown once.
				{:else if confirmation?.kind === "disable"}
					Its credential stops working immediately. Its saved authority stays available if you
					enable it later.
				{:else if confirmation?.kind === "enable"}
					Its latest saved authority is restored and a fresh credential is shown once.
				{/if}
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			{#if busy}
				<Button variant="outline" disabled>Cancel</Button>
			{:else}
				<AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
			{/if}
			<AlertDialog.Action
				variant={confirmation?.kind === "disable" ? "destructive" : "default"}
				disabled={busy}
				onclick={confirm}
			>
				{#if mutation.kind === "rotating"}
					Rotating…
				{:else if mutation.kind === "disabling"}
					Disabling…
				{:else if mutation.kind === "enabling"}
					Enabling…
				{:else if confirmation?.kind === "rotate"}
					Rotate credential
				{:else if confirmation?.kind === "disable"}
					Disable agent
				{:else if confirmation?.kind === "enable"}
					Enable agent
				{/if}
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>

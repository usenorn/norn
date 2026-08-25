<script lang="ts">
	import { invalidate } from "$app/navigation";
	import { page } from "$app/state";
	import ArrowLeft from "@lucide/svelte/icons/arrow-left";
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import Gauge from "@lucide/svelte/icons/gauge";
	import KeyRound from "@lucide/svelte/icons/key-round";
	import Power from "@lucide/svelte/icons/power";
	import RotateCw from "@lucide/svelte/icons/rotate-cw";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import Users from "@lucide/svelte/icons/users";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as AlertDialog from "$lib/components/ui/alert-dialog/index.js";
	import * as Tabs from "$lib/components/ui/tabs/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Skeleton } from "$lib/components/ui/skeleton/index.js";
	import Tag from "$lib/components/norn/tag.svelte";
	import ActivityFeedView from "$lib/activity/activity-feed.svelte";
	import AgentCapabilitiesPanel from "$lib/agents/agent-capabilities-panel.svelte";
	import AgentCapabilityDialog from "$lib/agents/agent-capability-dialog.svelte";
	import AgentIcon from "$lib/agents/agent-icon.svelte";
	import CredentialDialog from "$lib/agents/credential-dialog.svelte";
	import { api } from "$lib/api";
	import { keys } from "$lib/api/keys";
	import { holdShortcuts } from "$lib/shortcuts/registry.svelte";
	import { onDate, onDateAndTime } from "$lib/time";
	import {
		agentScopeLabels,
		agentsPath,
		type Agent,
	} from "$lib/agents/agents";
	import {
		withCapability,
		type AgentCapabilities,
		type AgentCapabilityDraft,
		type AgentCapabilityKind,
	} from "$lib/agents/agent-capabilities";
	import {
		agentLifecycleFailureMessage,
		type AgentLifecycleAction,
		type AgentLifecycleFailure,
	} from "$lib/agents/agent-record";
	import { runnersPath } from "$lib/runners/runners";
	import type { ActivityFeed } from "$lib/activity/activity";
	import { agentRecordPreviewStates, type AgentDetailTab } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV
			? agentRecordPreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined
	);

	let loadedActivity = $state.raw<ActivityFeed | null>(null);
	let capabilityOverride = $state.raw<AgentCapabilities | null>(null);
	let selectedTab = $state<AgentDetailTab>("overview");
	let capabilityDialogKind = $state<AgentCapabilityKind>("skill");
	let capabilityDialogOpen = $state(false);
	let working = $state(false);
	let mutation = $state<AgentLifecycleAction | null>(null);
	let confirmation = $state<AgentLifecycleAction | null>(null);
	let lifecycleFailure = $state<AgentLifecycleFailure | null>(null);
	let issuedAgent = $state.raw<Agent | null>(null);
	let issuedValue = $state("");
	let stateAgentId = $state<string | undefined>(page.params.agentId);

	const workspace = $derived(data.workspace);
	const record = $derived(preview?.record ?? data.record);
	const agent = $derived(record.kind === "ready" ? record.value : null);
	const activity = $derived<ActivityFeed>(loadedActivity ?? preview?.activity ?? data.activity);
	const capabilities = $derived<AgentCapabilities>(
		capabilityOverride ??
			preview?.capabilities ??
			(import.meta.env.DEV ? { kind: "empty" } : { kind: "unavailable" })
	);
	const teams = $derived(data.teams ?? []);
	const mutationAnnouncement = $derived(
		mutation === "rotate"
			? "Issuing a new credential."
			: mutation === "disable"
				? "Disabling the agent."
				: mutation === "enable"
					? "Enabling the agent."
					: ""
	);

	holdShortcuts(
		() => capabilityDialogOpen || confirmation !== null || issuedAgent !== null
	);

	$effect.pre(() => {
		const nextAgentId = page.params.agentId;

		if (nextAgentId === stateAgentId) return;

		stateAgentId = nextAgentId;
		loadedActivity = null;
		capabilityOverride = null;
		selectedTab = "overview";
		capabilityDialogKind = "skill";
		capabilityDialogOpen = false;
		working = false;
		mutation = null;
		confirmation = null;
		lifecycleFailure = null;
		issuedAgent = null;
		issuedValue = "";
	});

	$effect(() => {
		page.params.agentId;
		if (preview?.tab) selectedTab = preview.tab;
		if (preview?.dialog) {
			capabilityDialogKind = preview.dialog;
			capabilityDialogOpen = true;
		}
	});

	async function more() {
		if (activity.kind !== "ready" || !activity.nextCursor || !agent) return;

		working = true;

		try {
			const { data: next, error } = await api.GET(
				"/workspaces/{workspaceId}/agents/{agentId}/activity",
				{
					params: {
						path: { workspaceId: workspace.id, agentId: agent.agent.id },
						query: { limit: 50, cursor: activity.nextCursor },
					},
				}
			);

			if (error || !next) return;

			loadedActivity = {
				kind: "ready",
				events: [...activity.events, ...next.events],
				nextCursor: next.nextCursor,
			};
		} finally {
			working = false;
		}
	}

	async function refreshAgent() {
		if (!agent) return;

		await Promise.all([
			invalidate(keys.agent(workspace.id, agent.agent.id)),
			invalidate(keys.agents(workspace.id)),
		]);
	}

	function refreshAfterMutation() {
		void refreshAgent().catch(() => undefined);
	}

	function lifecycleProblem(error: object | undefined, status: number): AgentLifecycleFailure {
		if (status === 403) return { kind: "forbidden" };

		if (error && "code" in error) {
			if (error.code === "agent_owner_invalid") return { kind: "owner_invalid" };
			if (error.code === "agent_disabled") return { kind: "disabled" };
			if (error.code === "agent_active") return { kind: "active" };
			if (error.code === "agent_authority_missing") return { kind: "authority_missing" };
		}

		return { kind: "unavailable" };
	}

	async function rotateCredential() {
		if (!agent) return;

		mutation = "rotate";
		lifecycleFailure = null;

		try {
			const { data: issued, error, response } = await api.POST(
				"/workspaces/{workspaceId}/agents/{agentId}/credential",
				{ params: { path: { workspaceId: workspace.id, agentId: agent.agent.id } } }
			);

			if (error || !issued) {
				lifecycleFailure = lifecycleProblem(error, response.status);
				return;
			}

			confirmation = null;
			issuedAgent = issued.agent;
			issuedValue = issued.value;
			refreshAfterMutation();
		} catch {
			lifecycleFailure = { kind: "unavailable" };
		} finally {
			mutation = null;
		}
	}

	async function disableAgent() {
		if (!agent) return;

		mutation = "disable";
		lifecycleFailure = null;

		try {
			const { error, response } = await api.DELETE(
				"/workspaces/{workspaceId}/agents/{agentId}",
				{ params: { path: { workspaceId: workspace.id, agentId: agent.agent.id } } }
			);

			if (error) {
				lifecycleFailure = lifecycleProblem(error, response.status);
				return;
			}

			confirmation = null;
			refreshAfterMutation();
		} catch {
			lifecycleFailure = { kind: "unavailable" };
		} finally {
			mutation = null;
		}
	}

	async function enableAgent() {
		if (!agent) return;

		mutation = "enable";
		lifecycleFailure = null;

		try {
			const { data: issued, error, response } = await api.POST(
				"/workspaces/{workspaceId}/agents/{agentId}/enable",
				{ params: { path: { workspaceId: workspace.id, agentId: agent.agent.id } } }
			);

			if (error || !issued) {
				lifecycleFailure = lifecycleProblem(error, response.status);
				return;
			}

			confirmation = null;
			issuedAgent = issued.agent;
			issuedValue = issued.value;
			refreshAfterMutation();
		} catch {
			lifecycleFailure = { kind: "unavailable" };
		} finally {
			mutation = null;
		}
	}

	async function confirmLifecycle() {
		const action = confirmation;

		if (action === "rotate") await rotateCredential();
		if (action === "disable") await disableAgent();
		if (action === "enable") await enableAgent();

		confirmation = null;
	}

	function openCapabilityDialog(kind: AgentCapabilityKind) {
		capabilityDialogKind = kind;
		capabilityDialogOpen = true;
	}

	function addCapability(draft: AgentCapabilityDraft) {
		capabilityOverride = withCapability(capabilities, draft);
	}

	function closeCredential() {
		issuedAgent = null;
		issuedValue = "";
	}

	function when(instant: string): string {
		return onDateAndTime(instant, workspace.timezone);
	}

	function teamName(teamId: string): string {
		const team = teams.find((candidate) => candidate.id === teamId);

		return team ? `${team.key} · ${team.name}` : teamId;
	}
</script>

<svelte:head>
	<title>{agent?.agent.name ?? "Agent"} · {workspace.name} · Norn</title>
</svelte:head>

<div class="flex min-h-0 flex-1 flex-col">
	<div class="flex h-11 flex-none items-center justify-between gap-3 border-b border-line-subtle px-4">
		<a
			href={agentsPath(workspace.slug)}
			class="flex items-center gap-1.5 text-sm text-muted-foreground motion-control hover:text-ink-900"
		>
			<ArrowLeft class="size-4" aria-hidden="true" />
			<span>Agents</span>
		</a>
		<a
			href={runnersPath(workspace.slug)}
			class="text-xs text-muted-foreground motion-control hover:text-ink-900"
		>
			Runners
		</a>
	</div>

	<div class="flex-1 overflow-auto">
		<div
			class="mx-auto flex w-full max-w-5xl flex-col gap-5 px-4 py-6 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))]"
		>
			<p class="sr-only" aria-live="polite" aria-atomic="true">{mutationAnnouncement}</p>
			{#if record.kind === "loading"}
				<div class="flex flex-col gap-5" role="status" aria-busy="true" aria-label="Loading agent">
					<div class="flex items-start gap-3">
						<Skeleton class="size-9" />
						<div class="flex flex-1 flex-col gap-2">
							<Skeleton class="h-5 w-44" />
							<Skeleton class="h-3 w-full max-w-96" />
						</div>
					</div>
					<Skeleton class="h-9 w-full" />
					<div class="grid gap-4 md:grid-cols-2">
						<Skeleton class="h-56 w-full" />
						<Skeleton class="h-56 w-full" />
					</div>
				</div>
			{:else if record.kind === "missing"}
				<Alert.Root variant="muted">
					<CircleAlert aria-hidden="true" />
					<Alert.Title>That agent is not here</Alert.Title>
					<Alert.Description>It may have been removed with its workspace.</Alert.Description>
				</Alert.Root>
			{:else if record.kind === "forbidden"}
				<Alert.Root variant="muted">
					<CircleAlert aria-hidden="true" />
					<Alert.Title>You may not inspect this agent</Alert.Title>
					<Alert.Description>Ask a workspace administrator for access.</Alert.Description>
				</Alert.Root>
			{:else if record.kind === "authority_missing"}
				<Alert.Root variant="warning">
					<CircleAlert aria-hidden="true" />
					<Alert.Title>This agent has no authority to restore</Alert.Title>
					<Alert.Description>
						Return to the roster and register a replacement agent.
					</Alert.Description>
					<Alert.Action>
						<Button href={agentsPath(workspace.slug)} variant="secondary" size="sm">
							Return to agents
						</Button>
					</Alert.Action>
				</Alert.Root>
			{:else if record.kind === "unavailable"}
				<Alert.Root variant="destructive">
					<TriangleAlert aria-hidden="true" />
					<Alert.Title>Could not load the agent</Alert.Title>
					<Alert.Description>Check your connection and reload.</Alert.Description>
				</Alert.Root>
			{:else if agent}
				<section class="flex flex-col gap-4 border-b border-line-subtle pb-5 sm:flex-row sm:items-start sm:justify-between">
					<div class="flex min-w-0 items-start gap-3">
						<AgentIcon icon={agent.agent.icon} class="size-9" />
						<div class="min-w-0">
							<div class="flex flex-wrap items-center gap-2">
								<h1 class="truncate text-xl font-medium tracking-snug text-ink-900">{agent.agent.name}</h1>
								<Tag name={agent.agent.status === "active" ? "Active" : "Disabled"} />
							</div>
							<p class="mt-1 text-sm text-muted-foreground">
								Acts for {agent.ownerName || agent.ownerEmail || "somebody who has left"}
							</p>
						</div>
					</div>
					<div class="flex flex-none flex-wrap gap-2">
						{#if agent.agent.status === "active"}
							<Button
								variant="secondary"
								size="sm"
								disabled={mutation !== null}
								onclick={() => (confirmation = "rotate")}
							>
								<RotateCw aria-hidden="true" />
								New credential
							</Button>
							<Button
								variant="destructive"
								size="sm"
								disabled={mutation !== null}
								onclick={() => (confirmation = "disable")}
							>
								<Power aria-hidden="true" />
								Disable
							</Button>
						{:else}
							<Button
								size="sm"
								disabled={mutation !== null}
								onclick={() => (confirmation = "enable")}
							>
								<Power aria-hidden="true" />
								Enable agent
							</Button>
						{/if}
					</div>
				</section>

				{#if lifecycleFailure}
					<Alert.Root variant="destructive">
						<TriangleAlert aria-hidden="true" />
						<Alert.Title>That did not work</Alert.Title>
						<Alert.Description>{agentLifecycleFailureMessage(lifecycleFailure)}</Alert.Description>
					</Alert.Root>
				{/if}

				<Tabs.Root bind:value={selectedTab} class="gap-0">
					<Tabs.List variant="line" class="w-full justify-start overflow-x-auto">
						<Tabs.Trigger value="overview" class="flex-none">Overview</Tabs.Trigger>
						<Tabs.Trigger value="capabilities" class="flex-none">Capabilities</Tabs.Trigger>
						<Tabs.Trigger value="activity" class="flex-none">Activity</Tabs.Trigger>
					</Tabs.List>

					<Tabs.Content value="overview" class="pt-5">
						<div class="grid gap-5 lg:grid-cols-[minmax(0,3fr)_minmax(16rem,2fr)]">
							<section class="min-w-0">
								<h2 class="text-md font-medium tracking-snug text-ink-900">Authority</h2>
								<p class="mt-1 text-sm leading-normal text-muted-foreground text-pretty">
									Read-only summary of what the latest credential permits.
								</p>

								<div class="mt-3 border border-line-subtle bg-paper-0">
									<div class="border-b border-line-subtle p-4">
										<div class="flex items-center gap-2 text-sm font-medium text-ink-900">
											<KeyRound aria-hidden="true" />
											Permissions
										</div>
										<ul class="mt-3 divide-y divide-line-subtle border border-line-subtle bg-paper-1">
											{#each agent.authority.scopes as scope (scope)}
												<li class="flex flex-col gap-0.5 p-2.5 sm:flex-row sm:items-center sm:justify-between sm:gap-3">
													<span class="text-sm text-ink-900">{agentScopeLabels[scope] ?? scope}</span>
													<code class="font-mono text-xs text-muted-foreground">{scope}</code>
												</li>
											{/each}
										</ul>
									</div>

									<div class="p-4">
										<div class="flex items-center gap-2 text-sm font-medium text-ink-900">
											<Users aria-hidden="true" />
											Team reach
										</div>
										{#if agent.authority.allTeams}
											<p class="mt-2 text-sm text-muted-foreground">Every team its owner may reach.</p>
										{:else}
											<ul class="mt-2 flex flex-col gap-1">
												{#each agent.authority.teamIds as teamId (teamId)}
													<li class="font-mono text-xs text-muted-foreground">{teamName(teamId)}</li>
												{/each}
											</ul>
										{/if}
									</div>
								</div>
							</section>

							<div class="flex min-w-0 flex-col gap-4">
								<section class="border border-line-subtle bg-paper-0 p-4">
									<div class="flex items-center gap-2 text-sm font-medium text-ink-900">
										<Gauge aria-hidden="true" />
										Action allowance
									</div>
									<p class="mt-2 text-2xl font-medium tracking-snug text-ink-900">{agent.agent.actionLimit}</p>
									<p class="text-xs text-muted-foreground">changes each minute</p>
								</section>

								<section class="border border-line-subtle bg-paper-0 p-4">
									<h2 class="text-sm font-medium tracking-snug text-ink-900">Record</h2>
									<dl class="mt-3 grid gap-3 text-sm">
										<div>
											<dt class="text-xs text-muted-foreground">Owner</dt>
											<dd class="mt-0.5 text-ink-900">{agent.ownerName || agent.ownerEmail}</dd>
										</div>
										<div>
											<dt class="text-xs text-muted-foreground">Registered</dt>
											<dd class="mt-0.5 text-ink-900">{onDate(agent.agent.createdAt, workspace.timezone)}</dd>
										</div>
										{#if agent.agent.disabledAt}
											<div>
												<dt class="text-xs text-muted-foreground">Disabled</dt>
												<dd class="mt-0.5 text-ink-900">{onDate(agent.agent.disabledAt, workspace.timezone)}</dd>
											</div>
										{/if}
									</dl>
								</section>
							</div>
						</div>
					</Tabs.Content>

					<Tabs.Content value="capabilities" class="pt-5">
						<div class="flex flex-col gap-4">
							{#if import.meta.env.DEV && capabilities.kind !== "unavailable"}
								<Alert.Root variant="muted">
									<CircleAlert aria-hidden="true" />
									<Alert.Title>Development draft</Alert.Title>
									<Alert.Description>
										Changes here stay in this browser session. No runtime capability API exists yet.
									</Alert.Description>
								</Alert.Root>
							{/if}
							<AgentCapabilitiesPanel
								{capabilities}
								canDraft={import.meta.env.DEV}
								onadd={openCapabilityDialog}
							/>
						</div>
					</Tabs.Content>

					<Tabs.Content value="activity" class="pt-5">
						<div class="flex flex-col gap-4">
							<p class="text-sm leading-normal text-muted-foreground text-pretty">
								Everything this agent has done across the work it could reach. The computers it
								acts on are listed on the
								<a href={runnersPath(workspace.slug)} class="text-link underline-offset-2 hover:underline">runners screen</a>.
							</p>
							<ActivityFeedView
								feed={activity}
								{when}
								{working}
								emptyLine="This agent has not done anything yet."
								onmore={more}
							/>
						</div>
					</Tabs.Content>
				</Tabs.Root>
			{/if}
		</div>
	</div>
</div>

{#if import.meta.env.DEV}
	<AgentCapabilityDialog
		bind:open={capabilityDialogOpen}
		kind={capabilityDialogKind}
		onadd={addCapability}
	/>
{/if}

<CredentialDialog
	open={issuedAgent !== null}
	agent={issuedAgent}
	value={issuedValue}
	origin={page.url.origin}
	workspaceName={workspace.name}
	rotated
	onclose={closeCredential}
/>

<AlertDialog.Root
	open={confirmation !== null}
	onOpenChange={(open) => {
		if (!open && mutation === null) confirmation = null;
	}}
>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>
				{confirmation === "disable"
					? `Disable ${agent?.agent.name}?`
					: confirmation === "enable"
						? `Enable ${agent?.agent.name}?`
						: `Issue a new credential for ${agent?.agent.name}?`}
			</AlertDialog.Title>
			<AlertDialog.Description>
				{confirmation === "disable"
					? "Every current credential stops working immediately."
					: confirmation === "enable"
						? "The latest previous authority is restored and a new one-time credential is issued."
						: "The current credential stops working as soon as the new one is issued."}
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			{#if mutation !== null}
				<Button variant="outline" disabled>Cancel</Button>
			{:else}
				<AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
			{/if}
			<AlertDialog.Action
				variant={confirmation === "disable" ? "destructive" : "default"}
				disabled={mutation !== null}
				onclick={confirmLifecycle}
			>
				{mutation === "disable"
					? "Disabling…"
					: mutation === "enable"
						? "Enabling…"
						: mutation === "rotate"
							? "Issuing…"
							: confirmation === "disable"
								? "Disable agent"
								: confirmation === "enable"
									? "Enable agent"
									: "Issue credential"}
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>

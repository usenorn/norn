<script lang="ts">
	import { invalidate } from "$app/navigation";
	import { page } from "$app/state";
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import Server from "@lucide/svelte/icons/server";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { api } from "$lib/api";
	import { keys } from "$lib/api/keys";
	import { agentsPath } from "$lib/agents/agents";
	import AgentGroup from "$lib/runners/agent-group.svelte";
	import ConnectDialog from "$lib/runners/connect-dialog.svelte";
	import {
		failureMessage,
		liveRefreshMs,
		machineFailure,
		type Codebase,
		type Runner,
		type RunnerFailure,
		type RunnersView,
	} from "$lib/runners/runners";
	import { runnersPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV
			? runnersPreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined
	);

	let failure = $state.raw<RunnerFailure | null>(null);
	let busyMachine = $state("");
	let busyCodebase = $state("");
	let copied = $state("");
	let connecting = $state(false);

	const workspace = $derived(page.data.workspace);
	const view = $derived<RunnersView>(preview?.view ?? data.view);
	const shown = $derived(preview?.failure ?? failure);
	const groups = $derived(view.kind === "ready" || view.kind === "empty" ? view.groups : []);
	const agents = $derived(preview?.agents ?? data.agents);

	$effect(() => {
		if (view.kind !== "ready") return;

		const timer = setInterval(() => {
			void invalidate(keys.page(page.route.id));
		}, liveRefreshMs);

		return () => clearInterval(timer);
	});

	async function copy(text: string) {
		try {
			await navigator.clipboard.writeText(text);
			copied = text;
		} catch {
			copied = "";
		}
	}

	async function settle(work: () => Promise<{ error?: unknown }>, machineId: string, codebaseId: string) {
		busyMachine = machineId;
		busyCodebase = codebaseId;
		failure = null;

		try {
			const { error } = await work();

			if (error) {
				failure = machineFailure(error as { status?: number });

				return;
			}

			await invalidate(keys.page(page.route.id));
		} catch {
			failure = { kind: "unavailable" };
		} finally {
			busyMachine = "";
			busyCodebase = "";
		}
	}

	function pause(machine: Runner) {
		void settle(
			() =>
				api.POST("/workspaces/{workspaceId}/runners/{runnerId}/pause", {
					params: { path: { workspaceId: workspace.id, runnerId: machine.id } },
				}),
			machine.id,
			""
		);
	}

	function resume(machine: Runner) {
		void settle(
			() =>
				api.POST("/workspaces/{workspaceId}/runners/{runnerId}/resume", {
					params: { path: { workspaceId: workspace.id, runnerId: machine.id } },
				}),
			machine.id,
			""
		);
	}

	function revoke(machine: Runner) {
		void settle(
			() =>
				api.DELETE("/workspaces/{workspaceId}/runners/{runnerId}", {
					params: { path: { workspaceId: workspace.id, runnerId: machine.id } },
				}),
			machine.id,
			""
		);
	}

	function disconnect(codebase: Codebase) {
		void settle(
			() =>
				api.DELETE("/workspaces/{workspaceId}/agents/{agentId}/codebases/{codebaseId}", {
					params: {
						path: {
							workspaceId: workspace.id,
							agentId: codebase.agentId,
							codebaseId: codebase.id,
						},
					},
				}),
			"",
			codebase.id
		);
	}
</script>

<svelte:head><title>Runners · {workspace.name} · Norn</title></svelte:head>

<div class="flex min-h-0 flex-1 flex-col">
	<div class="flex h-11 flex-none items-center justify-between gap-3 border-b border-line-subtle px-4">
		<div class="flex min-w-0 items-center gap-2">
			<Server class="size-icon-toolbar flex-none text-muted-foreground" aria-hidden="true" />
			<h1 class="truncate text-sm font-medium tracking-snug text-ink-900">Runners</h1>
		</div>
		{#if view.kind === "ready" || view.kind === "empty"}
			<Button size="sm" onclick={() => (connecting = true)}>Connect a machine</Button>
		{/if}
	</div>

	<div class="relative flex-1 overflow-auto">
		<div
			class="mx-auto flex w-full max-w-180 flex-col gap-5 px-4 py-6 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))]"
		>
			<p class="text-sm leading-normal text-muted-foreground text-pretty">
				A runner is one computer bound to one of your agents. It is where delegated work actually
				runs: it holds the folders, starts the coding agent and puts the change up for review.
				Machines are grouped by the agent they act as, because a runner is not an identity of its
				own.
			</p>

			{#if shown}
				<Alert.Root variant="destructive">
					<TriangleAlert aria-hidden="true" />
					<Alert.Title>That did not work</Alert.Title>
					<Alert.Description>{failureMessage(shown)}</Alert.Description>
				</Alert.Root>
			{/if}

			{#if view.kind === "loading"}
				<p class="text-sm text-muted-foreground">Reading what is connected…</p>
			{:else if view.kind === "forbidden"}
				<Alert.Root variant="muted">
					<CircleAlert aria-hidden="true" />
					<Alert.Title>You may not read the machines here</Alert.Title>
					<Alert.Description>
						You see the machines of agents you own, and an administrator of {workspace.name} sees
						every machine. Ask one of them if a machine you expected is missing.
					</Alert.Description>
				</Alert.Root>
			{:else if view.kind === "unavailable"}
				<Alert.Root variant="destructive">
					<TriangleAlert aria-hidden="true" />
					<Alert.Title>Could not load the machines</Alert.Title>
					<Alert.Description>Check your connection and reload.</Alert.Description>
				</Alert.Root>
			{:else if view.kind === "no_agents"}
				<div
					class="flex flex-col items-center gap-3 rounded-lg border border-line-default bg-paper-0 px-6 py-10 text-center"
				>
					<Server class="size-5 text-muted-foreground" aria-hidden="true" />
					<p class="max-w-100 text-sm leading-normal text-muted-foreground text-pretty">
						A machine acts as an agent, and this workspace has none yet. Register an agent first;
						a computer can then be bound to it, and delegated work runs there.
					</p>
					<Button variant="secondary" size="sm" href={agentsPath(workspace.slug)}>
						Register an agent
					</Button>
				</div>
			{:else}
				{#each groups as group (group.agent.id)}
					<AgentGroup
						{group}
						workspaceSlug={workspace.slug}
						now={page.data.now}
						timezone={workspace.timezone}
						{busyMachine}
						{busyCodebase}
						{copied}
						oncopy={copy}
						onconnect={() => (connecting = true)}
						onpause={pause}
						onresume={resume}
						onrevoke={revoke}
						ondisconnect={disconnect}
					/>
				{/each}
			{/if}
		</div>
	</div>
</div>

<ConnectDialog
	bind:open={connecting}
	workspaceId={workspace.id}
	{agents}
	{copied}
	oncopy={copy}
/>

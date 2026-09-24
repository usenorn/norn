<script lang="ts">
	import Cable from "@lucide/svelte/icons/cable";
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import Puzzle from "@lucide/svelte/icons/puzzle";
	import { api } from "$lib/api";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Dialog from "$lib/components/ui/dialog/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import {
		agentLibraryPath,
		capabilityFailure,
		capabilityFailureMessage,
		mcpServerSummary,
		type AgentCapabilityFailure,
		type AgentCapabilityKind,
		type AgentMcpServer,
		type AgentSkill,
	} from "./agent-capabilities";

	let {
		open = $bindable(false),
		kind,
		workspaceId,
		workspaceSlug,
		agentId,
		skills,
		servers,
		onattached,
	}: {
		open?: boolean;
		kind: AgentCapabilityKind;
		workspaceId: string;
		workspaceSlug: string;
		agentId: string;
		skills: AgentSkill[];
		servers: AgentMcpServer[];
		onattached: () => void;
	} = $props();

	let attaching = $state<string | null>(null);
	let failure = $state<AgentCapabilityFailure | null>(null);

	const items = $derived(
		kind === "skill"
			? skills.map((skill) => ({ id: skill.id, name: skill.name, line: skill.description }))
			: servers.map((server) => ({ id: server.id, name: server.name, line: mcpServerSummary(server) }))
	);

	$effect(() => {
		if (open) failure = null;
	});

	async function attach(id: string) {
		attaching = id;
		failure = null;

		try {
			const { error, response } =
				kind === "skill"
					? await api.PUT("/workspaces/{workspaceId}/agents/{agentId}/library-skills/{skillId}", {
							params: { path: { workspaceId, agentId, skillId: id } },
						})
					: await api.PUT("/workspaces/{workspaceId}/agents/{agentId}/library-mcp-servers/{serverId}", {
							params: { path: { workspaceId, agentId, serverId: id } },
						});

			if (error) {
				failure = capabilityFailure(error, response.status);

				return;
			}

			onattached();
			open = false;
		} catch {
			failure = { kind: "unavailable" };
		} finally {
			attaching = null;
		}
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content variant="scrollable" class="sm:max-w-120">
		<Dialog.Header>
			<Dialog.Title>{kind === "skill" ? "Use a library skill" : "Use a library MCP server"}</Dialog.Title>
			<Dialog.Description>
				The agent uses the library copy, so a change made in the library reaches every agent that uses it.
			</Dialog.Description>
		</Dialog.Header>

		{#if failure}
			<Alert.Root variant="destructive">
				<CircleAlert aria-hidden="true" />
				<Alert.Title>That did not work</Alert.Title>
				<Alert.Description>{capabilityFailureMessage(failure)}</Alert.Description>
			</Alert.Root>
		{/if}

		{#if items.length === 0}
			<p class="text-sm leading-normal text-muted-foreground text-pretty">
				{kind === "skill"
					? "Every library skill is already in use here, or the library has none."
					: "Every library MCP server is already in use here, or the library has none."}
			</p>
			<a href={agentLibraryPath(workspaceSlug)} class="w-fit text-sm text-link underline-offset-2 hover:underline">
				Open the agent library
			</a>
		{:else}
			<ul class="divide-y divide-line-subtle border border-line-subtle bg-paper-1">
				{#each items as item (item.id)}
					<li class="flex items-start gap-3 p-3">
						{#if kind === "skill"}
							<Puzzle class="mt-0.5 size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
						{:else}
							<Cable class="mt-0.5 size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
						{/if}
						<div class="min-w-0 flex-1">
							<p class="text-sm text-ink-900">{item.name}</p>
							<p class="mt-0.5 line-clamp-2 text-xs text-muted-foreground">{item.line}</p>
						</div>
						<Button
							variant="secondary"
							size="sm"
							disabled={attaching !== null}
							onclick={() => attach(item.id)}
						>
							{attaching === item.id ? "Adding…" : "Use"}
						</Button>
					</li>
				{/each}
			</ul>
		{/if}

		<Dialog.Footer>
			<Button variant="secondary" disabled={attaching !== null} onclick={() => (open = false)}>Close</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

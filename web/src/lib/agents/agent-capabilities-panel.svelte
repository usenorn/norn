<script lang="ts">
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import Library from "@lucide/svelte/icons/library";
	import Plus from "@lucide/svelte/icons/plus";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Skeleton } from "$lib/components/ui/skeleton/index.js";
	import AgentCapabilitySection from "./agent-capability-section.svelte";
	import McpServerRow from "./mcp-server-row.svelte";
	import SkillRow from "./skill-row.svelte";
	import type {
		CapabilityRows,
		McpServerAction,
		McpServerEntry,
		SkillAction,
		SkillEntry,
	} from "./agent-capabilities";

	let {
		rows,
		scope,
		canManage,
		canManageLibrary,
		busyId,
		locked,
		connectForm,
		onaddskill,
		onaddserver,
		onattach,
		onskill,
		onserver,
		onconnect,
	}: {
		rows: CapabilityRows;
		scope: "agent" | "library";
		canManage: boolean;
		canManageLibrary: boolean;
		busyId: string | null;
		locked: boolean;
		connectForm: string;
		onaddskill: () => void;
		onaddserver: () => void;
		onattach?: (kind: "skill" | "mcp") => void;
		onskill: (entry: SkillEntry, action: SkillAction) => void;
		onserver: (entry: McpServerEntry, action: McpServerAction) => void;
		onconnect: (serverId: string) => void;
	} = $props();

	const agentScope = $derived(scope === "agent");
</script>

{#if rows.kind === "loading"}
	<div class="flex flex-col gap-3" role="status" aria-busy="true" aria-label="Loading skills and MCP servers">
		{#each Array(2) as _}
			<div class="flex min-h-28 flex-col justify-between gap-4 border border-line-subtle p-4">
				<div class="flex flex-col gap-2">
					<Skeleton class="h-4 w-28" />
					<Skeleton class="h-3 w-full max-w-100" />
				</div>
				<Skeleton class="h-3 w-40" />
			</div>
		{/each}
	</div>
{:else if rows.kind === "forbidden"}
	<Alert.Root variant="muted">
		<CircleAlert aria-hidden="true" />
		<Alert.Title>You may not see these</Alert.Title>
		<Alert.Description>Ask the agent's owner or a workspace administrator.</Alert.Description>
	</Alert.Root>
{:else if rows.kind === "unavailable"}
	<Alert.Root variant="destructive">
		<TriangleAlert aria-hidden="true" />
		<Alert.Title>Could not load skills and MCP servers</Alert.Title>
		<Alert.Description>Check your connection and reload.</Alert.Description>
	</Alert.Root>
{:else}
	<div class="border border-line-subtle bg-paper-0">
		<AgentCapabilitySection
			title="Skills"
			description={agentScope
				? "Instructions and scripts the agent loads when a task calls for them."
				: "Skills any agent in this workspace can be given."}
			emptyLine={agentScope ? "This agent has no skills yet." : "The library has no skills yet."}
			empty={rows.skills.length === 0}
		>
			{#snippet actions()}
				{#if canManage}
					{#if onattach}
						<Button variant="ghost" size="sm" onclick={() => onattach("skill")}>
							<Library aria-hidden="true" />
							From library
						</Button>
					{/if}
					<Button variant="secondary" size="sm" onclick={onaddskill}>
						<Plus aria-hidden="true" />
						Add skill
					</Button>
				{/if}
			{/snippet}
			<ul class="divide-y divide-line-subtle border border-line-subtle bg-paper-1">
				{#each rows.skills as entry (entry.skill.id)}
					<SkillRow
						skill={entry.skill}
						shared={entry.shared}
						usedBy={entry.usedBy}
						busy={locked || busyId === entry.skill.id}
						canManage={entry.shared ? canManageLibrary : canManage}
						canDetach={canManage}
						onaction={(action) => onskill(entry, action)}
					/>
				{/each}
			</ul>
		</AgentCapabilitySection>

		<AgentCapabilitySection
			title="MCP servers"
			description={agentScope
				? "Tools the agent can call during a run, from a command on its runner or a remote server."
				: "MCP servers any agent in this workspace can be given."}
			emptyLine={agentScope ? "This agent has no MCP servers yet." : "The library has no MCP servers yet."}
			empty={rows.servers.length === 0}
		>
			{#snippet actions()}
				{#if canManage}
					{#if onattach}
						<Button variant="ghost" size="sm" onclick={() => onattach("mcp")}>
							<Library aria-hidden="true" />
							From library
						</Button>
					{/if}
					<Button variant="secondary" size="sm" onclick={onaddserver}>
						<Plus aria-hidden="true" />
						Add MCP server
					</Button>
				{/if}
			{/snippet}
			<ul class="divide-y divide-line-subtle border border-line-subtle bg-paper-1">
				{#each rows.servers as entry (entry.server.id)}
					<McpServerRow
						server={entry.server}
						shared={entry.shared}
						usedBy={entry.usedBy}
						busy={locked || busyId === entry.server.id}
						canManage={entry.shared ? canManageLibrary : canManage}
						canDetach={canManage}
						{connectForm}
						{onconnect}
						onaction={(action) => onserver(entry, action)}
					/>
				{/each}
			</ul>
		</AgentCapabilitySection>
	</div>
{/if}

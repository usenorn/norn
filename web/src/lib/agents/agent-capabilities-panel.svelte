<script lang="ts">
	import Cable from "@lucide/svelte/icons/cable";
	import Puzzle from "@lucide/svelte/icons/puzzle";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import { Skeleton } from "$lib/components/ui/skeleton/index.js";
	import Tag from "$lib/components/norn/tag.svelte";
	import AgentCapabilitySection from "./agent-capability-section.svelte";
	import type { AgentCapabilities, AgentCapabilityKind } from "./agent-capabilities";

	let {
		capabilities,
		canDraft,
		onadd,
	}: {
		capabilities: AgentCapabilities;
		canDraft: boolean;
		onadd: (kind: AgentCapabilityKind) => void;
	} = $props();

	const skills = $derived(capabilities.kind === "ready" ? capabilities.skills : []);
	const mcpServers = $derived(capabilities.kind === "ready" ? capabilities.mcpServers : []);
</script>

{#if capabilities.kind === "unavailable"}
	<Alert.Root variant="muted">
		<TriangleAlert aria-hidden="true" />
		<Alert.Title>Capability management is not available yet</Alert.Title>
		<Alert.Description>
			Skills and MCP servers need runtime-backed APIs before they can be configured here.
		</Alert.Description>
	</Alert.Root>
{:else if capabilities.kind === "loading"}
	<div
		class="flex flex-col gap-3"
		role="status"
		aria-busy="true"
		aria-label="Loading agent capabilities"
	>
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
{:else}
	<div class="border border-line-subtle bg-paper-0">
		<AgentCapabilitySection
			title="Skills"
			description="Reusable instructions the runtime may load for this agent."
			emptyLine="No draft skills are configured."
			empty={skills.length === 0}
			action="Add skill"
			onadd={canDraft ? () => onadd("skill") : undefined}
		>
			{#if skills.length > 0}
				<ul class="divide-y divide-line-subtle border border-line-subtle bg-paper-1">
					{#each skills as skill}
						<li class="flex min-w-0 items-start gap-3 p-3">
							<Puzzle class="mt-0.5 size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
							<div class="min-w-0 flex-1">
								<div class="flex flex-wrap items-center gap-2">
									<p class="text-sm text-ink-900">{skill.name}</p>
									<Tag name="Draft" />
								</div>
								<p class="mt-0.5 font-mono text-xs break-all text-muted-foreground">{skill.source}</p>
							</div>
						</li>
					{/each}
				</ul>
			{/if}
		</AgentCapabilitySection>

		<AgentCapabilitySection
			title="MCP servers"
			description="External tools the runtime may expose to this agent."
			emptyLine="No draft MCP servers are configured."
			empty={mcpServers.length === 0}
			action="Add MCP server"
			onadd={canDraft ? () => onadd("mcp") : undefined}
		>
			{#if mcpServers.length > 0}
				<ul class="divide-y divide-line-subtle border border-line-subtle bg-paper-1">
					{#each mcpServers as server}
						<li class="flex min-w-0 items-start gap-3 p-3">
							<Cable class="mt-0.5 size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
							<div class="min-w-0 flex-1">
								<div class="flex flex-wrap items-center gap-2">
									<p class="text-sm text-ink-900">{server.name}</p>
									<Tag name="Draft" />
								</div>
								<p class="mt-0.5 font-mono text-xs break-all text-muted-foreground">
									{server.transport === "stdio"
										? [server.command, ...server.args].join(" ")
										: server.url}
								</p>
								<p class="mt-0.5 text-xs text-muted-foreground">
									{server.transport === "stdio"
										? "Local process · stdio"
										: server.auth === "bearer"
											? "Remote URL · bearer authentication"
											: "Remote URL · no authentication"}
								</p>
							</div>
						</li>
					{/each}
				</ul>
			{/if}
		</AgentCapabilitySection>
	</div>
{/if}

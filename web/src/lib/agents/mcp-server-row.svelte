<script lang="ts">
	import Cable from "@lucide/svelte/icons/cable";
	import LogIn from "@lucide/svelte/icons/log-in";
	import LogOut from "@lucide/svelte/icons/log-out";
	import MoreHorizontal from "@lucide/svelte/icons/more-horizontal";
	import Pencil from "@lucide/svelte/icons/pencil";
	import Trash2 from "@lucide/svelte/icons/trash-2";
	import Unplug from "@lucide/svelte/icons/unplug";
	import * as DropdownMenu from "$lib/components/ui/dropdown-menu/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import Tag from "$lib/components/norn/tag.svelte";
	import {
		mcpServerSummary,
		mcpServerTarget,
		mcpSignIn,
		type AgentMcpServer,
		type McpServerAction,
	} from "./agent-capabilities";

	let {
		server,
		shared,
		usedBy,
		busy,
		canManage,
		canDetach,
		connectForm,
		onconnect,
		onaction,
	}: {
		server: AgentMcpServer;
		shared: boolean;
		usedBy?: string[];
		busy: boolean;
		canManage: boolean;
		canDetach: boolean;
		connectForm: string;
		onconnect: (serverId: string) => void;
		onaction: (action: McpServerAction) => void;
	} = $props();

	const signIn = $derived(mcpSignIn(server));
	const summary = $derived(
		signIn.kind === "connected" || signIn.kind === "expired" || signIn.kind === "failed"
			? `${mcpServerSummary(server)} · ${signIn.issuer}`
			: mcpServerSummary(server)
	);
	const hasActions = $derived(canManage || (shared && canDetach));
</script>

<li class="flex min-w-0 flex-col gap-3 p-3 sm:flex-row sm:items-start" aria-busy={busy}>
	<div class="flex min-w-0 flex-1 items-start gap-3">
		<Cable class="mt-0.5 size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
		<div class="min-w-0 flex-1">
			<div class="flex flex-wrap items-center gap-2">
				<p class="text-sm text-ink-900">{server.name}</p>
				{#if shared}
					<Tag name="Library" color="violet" />
				{/if}
				{#if signIn.kind === "connected"}
					<Tag name="Signed in" color="cyan" />
				{:else if signIn.kind === "signed_out"}
					<Tag name="Needs sign-in" color="magenta" />
				{:else if signIn.kind === "expired" || signIn.kind === "failed"}
					<Tag name="Sign in again" color="magenta" />
				{/if}
			</div>
			<p class="mt-0.5 font-mono text-2xs break-all text-muted-foreground">{mcpServerTarget(server)}</p>
			<p class="mt-0.5 text-xs text-muted-foreground">{summary}</p>
			{#if signIn.kind === "failed"}
				<p class="mt-1 text-xs text-destructive">
					The provider stopped accepting the saved sign-in, so runs leave this server out until
					someone signs in again.
				</p>
			{:else if signIn.kind === "signed_out"}
				<p class="mt-1 text-xs text-muted-foreground">Runs leave this server out until someone signs in.</p>
			{/if}
			{#if usedBy}
				<p class="mt-1 text-xs text-muted-foreground">
					{usedBy.length === 0 ? "No agent uses it yet" : `Used by ${usedBy.join(", ")}`}
				</p>
			{/if}
		</div>
	</div>

	<div class="flex flex-none items-center gap-1 self-end sm:self-start">
		{#if canManage && signIn.kind !== "not_needed" && signIn.kind !== "connected"}
			<Button
				type="submit"
				form={connectForm}
				name="serverId"
				value={server.id}
				variant="secondary"
				size="sm"
				disabled={busy}
				onclick={() => onconnect(server.id)}
			>
				<LogIn aria-hidden="true" />
				{signIn.kind === "signed_out" ? "Sign in" : "Sign in again"}
			</Button>
		{/if}
		{#if hasActions}
			<DropdownMenu.Root>
				<DropdownMenu.Trigger disabled={busy}>
					{#snippet child({ props })}
						<Button {...props} variant="ghost" size="icon-sm" aria-label={`Actions for ${server.name}`}>
							<MoreHorizontal aria-hidden="true" />
						</Button>
					{/snippet}
				</DropdownMenu.Trigger>
				<DropdownMenu.Content align="end">
					{#if canManage}
						<DropdownMenu.Item onSelect={() => onaction("edit")}>
							<Pencil aria-hidden="true" />
							Edit
						</DropdownMenu.Item>
						{#if signIn.kind === "connected" || signIn.kind === "expired" || signIn.kind === "failed"}
							<DropdownMenu.Item onSelect={() => onaction("disconnect")}>
								<LogOut aria-hidden="true" />
								Sign out
							</DropdownMenu.Item>
						{/if}
					{/if}
					{#if shared && canDetach}
						<DropdownMenu.Item onSelect={() => onaction("detach")}>
							<Unplug aria-hidden="true" />
							Stop using
						</DropdownMenu.Item>
					{:else if canManage}
						<DropdownMenu.Item variant="destructive" onSelect={() => onaction("delete")}>
							<Trash2 aria-hidden="true" />
							Delete
						</DropdownMenu.Item>
					{/if}
				</DropdownMenu.Content>
			</DropdownMenu.Root>
		{/if}
	</div>
</li>

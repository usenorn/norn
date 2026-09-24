<script lang="ts">
	import { invalidate } from "$app/navigation";
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import type { Infer, SuperValidated } from "sveltekit-superforms";
	import { api } from "$lib/api";
	import { keys } from "$lib/api/keys";
	import { holdShortcuts } from "$lib/shortcuts/registry.svelte";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as AlertDialog from "$lib/components/ui/alert-dialog/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import AgentCapabilitiesPanel from "./agent-capabilities-panel.svelte";
	import LibraryPickerDialog from "./library-picker-dialog.svelte";
	import McpConnectForm from "./mcp-connect-form.svelte";
	import McpServerDialog from "./mcp-server-dialog.svelte";
	import SkillDialog from "./skill-dialog.svelte";
	import {
		capabilityFailure,
		capabilityFailureMessage,
		unattached,
		type AgentCapabilityFailure,
		type AgentCapabilityKind,
		type AgentLibraryListing,
		type CapabilityDialogPreview,
		type CapabilityRows,
		type McpConnectOutcome,
		type McpServerAction,
		type McpServerDialogMode,
		type McpServerEntry,
		type SkillAction,
		type SkillDialogMode,
		type SkillEntry,
	} from "./agent-capabilities";
	import { mcpConnectFormId, type mcpConnectSchema } from "./agent-capability-schemas";

	type Confirmation =
		| { kind: "delete-skill"; id: string; name: string; shared: boolean }
		| { kind: "detach-skill"; id: string; name: string }
		| { kind: "delete-server"; id: string; name: string; shared: boolean }
		| { kind: "detach-server"; id: string; name: string }
		| { kind: "disconnect"; id: string; name: string };

	let {
		workspace,
		agentId,
		rows,
		library,
		canManage,
		canManageLibrary,
		selfHosted,
		connectForm,
		outcome,
		dialog,
	}: {
		workspace: { id: string; slug: string };
		agentId?: string;
		rows: CapabilityRows;
		library?: AgentLibraryListing;
		canManage: boolean;
		canManageLibrary: boolean;
		selfHosted: boolean;
		connectForm: SuperValidated<Infer<typeof mcpConnectSchema>, AgentCapabilityFailure>;
		outcome: McpConnectOutcome;
		dialog?: CapabilityDialogPreview;
	} = $props();

	let skillOpen = $state(false);
	let skillMode = $state.raw<SkillDialogMode>({ kind: "add" });
	let serverOpen = $state(false);
	let serverMode = $state.raw<McpServerDialogMode>({ kind: "add" });
	let pickerOpen = $state(false);
	let pickerKind = $state<AgentCapabilityKind>("skill");
	let busyId = $state<string | null>(null);
	let failure = $state<AgentCapabilityFailure | null>(null);
	let confirmation = $state.raw<Confirmation | null>(null);
	let connecting = $state(false);
	let connector = $state<{ choose: (serverId: string) => void } | null>(null);

	holdShortcuts(() => skillOpen || serverOpen || pickerOpen || confirmation !== null);

	const listedSkills = $derived(rows.kind === "listed" ? rows.skills.map((entry) => entry.skill) : []);
	const listedServers = $derived(rows.kind === "listed" ? rows.servers.map((entry) => entry.server) : []);
	const librarySkills = $derived(
		library?.kind === "ready" ? unattached(library.library.skills.map((entry) => entry.skill), listedSkills) : []
	);
	const libraryServers = $derived(
		library?.kind === "ready"
			? unattached(library.library.mcpServers.map((entry) => entry.server), listedServers)
			: []
	);

	$effect(() => {
		if (dialog === "skill") skillOpen = true;
		if (dialog === "mcp") serverOpen = true;
		if (dialog === "picker-skill" || dialog === "picker-mcp") {
			pickerKind = dialog === "picker-skill" ? "skill" : "mcp";
			pickerOpen = true;
		}
	});

	async function refresh() {
		await Promise.all([
			agentId ? invalidate(keys.agentCapabilities(workspace.id, agentId)) : Promise.resolve(),
			invalidate(keys.agentLibrary(workspace.id)),
		]);
	}

	function saved() {
		failure = null;
		void refresh().catch(() => undefined);
	}

	function addSkill() {
		skillMode = { kind: "add" };
		skillOpen = true;
	}

	function addServer() {
		serverMode = { kind: "add" };
		serverOpen = true;
	}

	function attach(kind: AgentCapabilityKind) {
		pickerKind = kind;
		pickerOpen = true;
	}

	function onskill(entry: SkillEntry, action: SkillAction) {
		const { skill } = entry;

		switch (action) {
			case "edit":
				skillMode = { kind: "rewrite", skill };
				skillOpen = true;
				return;
			case "pull":
				void mutate(skill.id, () =>
					api.POST("/workspaces/{workspaceId}/agent-skills/{skillId}/pull", {
						params: { path: { workspaceId: workspace.id, skillId: skill.id } },
					})
				);
				return;
			case "detach":
				confirmation = { kind: "detach-skill", id: skill.id, name: skill.name };
				return;
			case "delete":
				confirmation = { kind: "delete-skill", id: skill.id, name: skill.name, shared: skill.library };
				return;
		}
	}

	function onserver(entry: McpServerEntry, action: McpServerAction) {
		const { server } = entry;

		switch (action) {
			case "edit":
				serverMode = { kind: "edit", server };
				serverOpen = true;
				return;
			case "disconnect":
				confirmation = { kind: "disconnect", id: server.id, name: server.name };
				return;
			case "detach":
				confirmation = { kind: "detach-server", id: server.id, name: server.name };
				return;
			case "delete":
				confirmation = { kind: "delete-server", id: server.id, name: server.name, shared: server.library };
				return;
		}
	}

	async function mutate(id: string, call: () => Promise<{ error?: unknown; response: Response }>) {
		busyId = id;
		failure = null;

		try {
			const { error, response } = await call();

			if (error) {
				failure = capabilityFailure(error, response.status);

				return false;
			}

			saved();

			return true;
		} catch {
			failure = { kind: "unavailable" };

			return false;
		} finally {
			busyId = null;
		}
	}

	async function confirm() {
		const chosen = confirmation;
		if (!chosen) return;

		const path = { workspaceId: workspace.id };

		switch (chosen.kind) {
			case "delete-skill":
				await mutate(chosen.id, () =>
					api.DELETE("/workspaces/{workspaceId}/agent-skills/{skillId}", {
						params: { path: { ...path, skillId: chosen.id } },
					})
				);
				break;
			case "delete-server":
				await mutate(chosen.id, () =>
					api.DELETE("/workspaces/{workspaceId}/agent-mcp-servers/{serverId}", {
						params: { path: { ...path, serverId: chosen.id } },
					})
				);
				break;
			case "disconnect":
				await mutate(chosen.id, () =>
					api.DELETE("/workspaces/{workspaceId}/agent-mcp-servers/{serverId}/connection", {
						params: { path: { ...path, serverId: chosen.id } },
					})
				);
				break;
			case "detach-skill":
				if (agentId) {
					await mutate(chosen.id, () =>
						api.DELETE("/workspaces/{workspaceId}/agents/{agentId}/library-skills/{skillId}", {
							params: { path: { ...path, agentId, skillId: chosen.id } },
						})
					);
				}
				break;
			case "detach-server":
				if (agentId) {
					await mutate(chosen.id, () =>
						api.DELETE("/workspaces/{workspaceId}/agents/{agentId}/library-mcp-servers/{serverId}", {
							params: { path: { ...path, agentId, serverId: chosen.id } },
						})
					);
				}
				break;
		}

		confirmation = null;
	}

	function connect(serverId: string) {
		connector?.choose(serverId);
	}

	const confirmationTitle = $derived.by(() => {
		switch (confirmation?.kind) {
			case "delete-skill":
				return `Delete ${confirmation.name}?`;
			case "delete-server":
				return `Delete ${confirmation.name}?`;
			case "detach-skill":
			case "detach-server":
				return `Stop using ${confirmation.name}?`;
			case "disconnect":
				return `Sign ${confirmation.name} out?`;
			default:
				return "";
		}
	});

	const confirmationBody = $derived.by(() => {
		switch (confirmation?.kind) {
			case "delete-skill":
			case "delete-server":
				return confirmation.shared
					? "It is removed from the library and from every agent that uses it. Runs already started keep their copy."
					: "The agent stops getting it on its next run. Runs already started keep their copy.";
			case "detach-skill":
			case "detach-server":
				return "It stays in the library for other agents. This agent stops getting it on its next run.";
			case "disconnect":
				return "Norn forgets the saved tokens. Runs leave the server out until someone signs in again.";
			default:
				return "";
		}
	});

	const confirmationAction = $derived(
		confirmation?.kind === "disconnect"
			? "Sign out"
			: confirmation?.kind === "detach-skill" || confirmation?.kind === "detach-server"
				? "Stop using"
				: "Delete"
	);
</script>

<div class="flex flex-col gap-4">
	<McpConnectForm bind:this={connector} data={connectForm} {outcome} bind:submitting={connecting} />

	{#if failure}
		<Alert.Root variant="destructive">
			<CircleAlert aria-hidden="true" />
			<Alert.Title>That did not work</Alert.Title>
			<Alert.Description>{capabilityFailureMessage(failure)}</Alert.Description>
		</Alert.Root>
	{/if}

	<AgentCapabilitiesPanel
		{rows}
		scope={agentId ? "agent" : "library"}
		{canManage}
		{canManageLibrary}
		{busyId}
		locked={connecting}
		connectForm={mcpConnectFormId}
		onaddskill={addSkill}
		onaddserver={addServer}
		onattach={agentId && library ? attach : undefined}
		{onskill}
		{onserver}
		onconnect={connect}
	/>
</div>

<SkillDialog bind:open={skillOpen} workspaceId={workspace.id} {agentId} mode={skillMode} onsaved={saved} />

<McpServerDialog
	bind:open={serverOpen}
	workspaceId={workspace.id}
	{agentId}
	mode={serverMode}
	{selfHosted}
	onsaved={saved}
/>

{#if agentId}
	<LibraryPickerDialog
		bind:open={pickerOpen}
		kind={pickerKind}
		workspaceId={workspace.id}
		workspaceSlug={workspace.slug}
		{agentId}
		skills={librarySkills}
		servers={libraryServers}
		onattached={saved}
	/>
{/if}

<AlertDialog.Root
	open={confirmation !== null}
	onOpenChange={(open) => {
		if (!open && busyId === null) confirmation = null;
	}}
>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>{confirmationTitle}</AlertDialog.Title>
			<AlertDialog.Description>{confirmationBody}</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			{#if busyId !== null}
				<Button variant="outline" disabled>Cancel</Button>
			{:else}
				<AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
			{/if}
			<AlertDialog.Action
				variant={confirmation?.kind === "delete-skill" || confirmation?.kind === "delete-server"
					? "destructive"
					: "default"}
				disabled={busyId !== null}
				onclick={confirm}
			>
				{busyId !== null ? "Working…" : confirmationAction}
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>

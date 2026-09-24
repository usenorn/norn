<script lang="ts">
	import { page } from "$app/state";
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import Library from "@lucide/svelte/icons/library";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import SettingsPage from "$lib/settings/settings-page.svelte";
	import AgentCapabilitiesManager from "$lib/agents/agent-capabilities-manager.svelte";
	import { agentsPath } from "$lib/agents/agents";
	import { connectOutcome, libraryRows, type AgentLibraryListing } from "$lib/agents/agent-capabilities";
	import { deploymentPreview } from "$lib/auth/preview";
	import { agentLibraryPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV ? agentLibraryPreviewStates[page.url.searchParams.get("state") ?? ""] : undefined
	);

	const workspace = $derived(data.workspace);
	const listing = $derived<AgentLibraryListing>(preview?.listing ?? data.listing);
	const agentNames = $derived(preview?.agentNames ?? data.agentNames);
	const outcome = $derived(preview?.outcome ?? connectOutcome(page.url));
	const selfHosted = $derived(deploymentPreview(page.url)?.selfHosted ?? data.selfHosted);
	const administrator = $derived(
		(preview?.role ?? data.members.find((member) => member.accountId === data.member.id)?.role) === "admin"
	);
	const rows = $derived(libraryRows(listing, (id) => agentNames[id] ?? "an agent you do not manage"));
</script>

<svelte:head>
	<title>Agent library · {workspace.name} · Norn</title>
</svelte:head>

<SettingsPage
	title="Agent library"
	description="Skills and MCP servers any agent in this workspace can be given."
	Icon={Library}
>
	<p class="text-sm leading-normal text-muted-foreground text-pretty">
		Add a skill or server here once, then give it to agents from their
		<a href={agentsPath(workspace.slug)} class="text-link underline-offset-2 hover:underline">settings</a>.
		A change made here reaches every agent that uses it on its next run.
	</p>

	{#if !administrator && listing.kind !== "forbidden"}
		<Alert.Root variant="muted">
			<CircleAlert aria-hidden="true" />
			<Alert.Title>Only workspace administrators change the library</Alert.Title>
			<Alert.Description>You can still give library items to the agents you own.</Alert.Description>
		</Alert.Root>
	{/if}

	<AgentCapabilitiesManager
		workspace={{ id: workspace.id, slug: workspace.slug }}
		{rows}
		canManage={administrator}
		canManageLibrary={administrator}
		{selfHosted}
		connectForm={data.connectForm}
		{outcome}
		dialog={preview?.dialog}
	/>
</SettingsPage>

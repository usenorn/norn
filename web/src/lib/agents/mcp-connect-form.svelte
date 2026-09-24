<script lang="ts">
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import { superForm, type Infer, type SuperValidated } from "sveltekit-superforms";
	import { zod4Client } from "sveltekit-superforms/adapters";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import {
		capabilityFailureMessage,
		type AgentCapabilityFailure,
		type McpConnectOutcome,
	} from "./agent-capabilities";
	import { mcpConnectFormId, mcpConnectSchema } from "./agent-capability-schemas";

	let {
		data,
		outcome,
		submitting = $bindable(false),
	}: {
		data: SuperValidated<Infer<typeof mcpConnectSchema>, AgentCapabilityFailure>;
		outcome: McpConnectOutcome;
		submitting?: boolean;
	} = $props();

	// svelte-ignore state_referenced_locally
	const form = superForm(data, { id: mcpConnectFormId, validators: zod4Client(mcpConnectSchema) });
	const { form: formData, enhance, message, submitting: sending } = form;

	$effect(() => {
		submitting = $sending;
	});

	export function choose(serverId: string) {
		$formData.serverId = serverId;
	}
</script>

<form method="POST" action="?/connect" id={mcpConnectFormId} use:enhance hidden>
	<input type="hidden" name="workspaceId" value={$formData.workspaceId} />
	<input type="hidden" name="returnTo" value={$formData.returnTo} />
</form>

<div aria-live="polite">
	{#if $message}
		<Alert.Root variant="destructive">
			<CircleAlert aria-hidden="true" />
			<Alert.Title>The sign-in could not start</Alert.Title>
			<Alert.Description>{capabilityFailureMessage($message)}</Alert.Description>
		</Alert.Root>
	{:else if outcome.kind === "connected"}
		<Alert.Root variant="success">
			<CircleAlert aria-hidden="true" />
			<Alert.Title>Signed in</Alert.Title>
			<Alert.Description>Runs now reach this server with the account you signed in with.</Alert.Description>
		</Alert.Root>
	{:else if outcome.kind === "refused"}
		<Alert.Root variant="warning">
			<CircleAlert aria-hidden="true" />
			<Alert.Title>The sign-in was cancelled</Alert.Title>
			<Alert.Description>The provider did not grant access. Sign in again when you are ready.</Alert.Description>
		</Alert.Root>
	{:else if outcome.kind === "expired"}
		<Alert.Root variant="warning">
			<CircleAlert aria-hidden="true" />
			<Alert.Title>That sign-in took too long</Alert.Title>
			<Alert.Description>It was already used, or it expired. Start it again.</Alert.Description>
		</Alert.Root>
	{:else if outcome.kind === "removed"}
		<Alert.Root variant="muted">
			<CircleAlert aria-hidden="true" />
			<Alert.Title>The server was removed</Alert.Title>
			<Alert.Description>It was deleted while the sign-in was open, so nothing was saved.</Alert.Description>
		</Alert.Root>
	{:else if outcome.kind === "failed"}
		<Alert.Root variant="destructive">
			<CircleAlert aria-hidden="true" />
			<Alert.Title>The sign-in did not finish</Alert.Title>
			<Alert.Description>
				The provider did not hand back a token Norn could use.
				{#if outcome.reference}
					Reference <code class="font-mono text-2xs">{outcome.reference}</code>.
				{/if}
			</Alert.Description>
		</Alert.Root>
	{/if}
</div>

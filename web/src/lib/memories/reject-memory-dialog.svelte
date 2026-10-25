<script lang="ts">
	import CircleX from "@lucide/svelte/icons/circle-x";
	import { defaults, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import { unknownLine } from "#lib/api/attempt.js";
	import * as Alert from "#lib/components/ui/alert/index.js";
	import * as Dialog from "#lib/components/ui/dialog/index.js";
	import * as Form from "#lib/components/ui/form/index.js";
	import { Button } from "#lib/components/ui/button/index.js";
	import { Textarea } from "#lib/components/ui/textarea/index.js";
	import { showToast } from "#lib/toast/toasts.js";
	import { memoryFailureMessage, readMemoryFailure, type Memory } from "./memories";
	import { rejectMemoryFormId, rejectMemorySchema } from "./memory-schema";
	import { refreshMemory, rejectMemory } from "./memory-writes";

	let {
		open = $bindable(false),
		memory,
		workspaceId,
	}: {
		open?: boolean;
		memory: Memory;
		workspaceId: string;
	} = $props();

	let failure = $state("");

	const form = superForm(defaults(zod4(rejectMemorySchema)), {
		id: rejectMemoryFormId,
		SPA: true,
		validators: zod4Client(rejectMemorySchema),
		resetForm: false,
		onSubmit: () => {
			failure = "";
		},
		onUpdate: async ({ form: entered }) => {
			if (!entered.valid) return;

			const outcome = await rejectMemory(workspaceId, memory, entered.data.reason);

			if (outcome.kind === "done") {
				open = false;
				showToast(`“${memory.statement}” is rejected`, { detail: "It is kept with your reason and never recalled." });
				await refreshMemory(workspaceId, memory.id);

				return;
			}

			failure = outcome.kind === "unknown" ? unknownLine : memoryFailureMessage(readMemoryFailure(outcome.problem));
		},
	});

	const { form: formData, enhance, submitting } = form;

	$effect(() => {
		if (!open) return;

		failure = "";
		$formData = { reason: "" };
	});
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title>Reject “{memory.statement}”?</Dialog.Title>
			<Dialog.Description>
				A rejected memory is never recalled by an agent. It is kept with its history and your reason.
			</Dialog.Description>
		</Dialog.Header>

		<form method="POST" use:enhance class="flex flex-col gap-4">
			{#if failure}
				<Alert.Root variant="destructive">
					<CircleX aria-hidden="true" />
					<Alert.Title>It was not rejected</Alert.Title>
					<Alert.Description>{failure}</Alert.Description>
				</Alert.Root>
			{/if}

			<Form.Field {form} name="reason">
				<Form.Control>
					{#snippet children({ props })}
						<Form.Label>Why</Form.Label>
						<Textarea {...props} bind:value={$formData.reason} disabled={$submitting} rows={3} />
					{/snippet}
				</Form.Control>
				<Form.FieldErrors />
			</Form.Field>

			<Dialog.Footer>
				<Button type="button" variant="ghost" disabled={$submitting} onclick={() => (open = false)}>
					Cancel
				</Button>
				<Button type="submit" variant="destructive" disabled={$submitting}>
					{$submitting ? "Rejecting" : "Reject"}
				</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>

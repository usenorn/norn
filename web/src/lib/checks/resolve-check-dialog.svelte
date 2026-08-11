<script lang="ts">
	import { defaults, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import { api } from "$lib/api";
	import * as Dialog from "$lib/components/ui/dialog/index.js";
	import * as Form from "$lib/components/ui/form/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Textarea } from "$lib/components/ui/textarea/index.js";
	import { checkReasonSchema } from "./check-schema";
	import {
		checkFailureMessage,
		readCheckFailure,
		type CheckFailure,
		type IssueCheck,
	} from "./checks";

	let {
		open,
		intent,
		workspaceId,
		issueId,
		check,
		onresolved,
		onclose,
	}: {
		open: boolean;
		intent: "waive" | "gap";
		workspaceId: string;
		issueId: string;
		check: IssueCheck | null;
		onresolved: () => void;
		onclose: () => void;
	} = $props();

	const copy = {
		waive: {
			title: "Set this criterion aside",
			description:
				"It stops standing in the way, and the record says you decided that and why. An agent can never do this.",
			label: "Why it does not apply",
			placeholder: "The endpoint this covered was dropped before release.",
			submit: "Waive it",
			working: "Waiving…",
		},
		gap: {
			title: "Record this as a gap",
			description:
				"Say plainly that it cannot be met. Norn files a child issue carrying it, so the work is not lost and this issue can finish honestly.",
			label: "What could not be done",
			placeholder: "The staging billing sandbox has been down all week, so nothing can exercise this.",
			submit: "Record the gap",
			working: "Filing…",
		},
	};

	const formId = "resolve-check-form";

	let failure = $state<CheckFailure | null>(null);

	const form = superForm(defaults(zod4(checkReasonSchema)), {
		id: formId,
		SPA: true,
		validators: zod4Client(checkReasonSchema),
		resetForm: true,
		onUpdate: async ({ form: entered }) => {
			if (!entered.valid || !check) return;

			failure = null;

			const path = { workspaceId, issueId, checkId: check.id };

			try {
				const { error } =
					intent === "waive"
						? await api.POST(
								"/workspaces/{workspaceId}/issues/{issueId}/checks/{checkId}/waiver",
								{ params: { path }, body: { reason: entered.data.reason } }
							)
						: await api.POST("/workspaces/{workspaceId}/issues/{issueId}/checks/{checkId}/gap", {
								params: { path },
								body: { reason: entered.data.reason },
							});

				if (error) {
					failure = readCheckFailure(error);

					return;
				}

				onresolved();
				onclose();
			} catch {
				failure = { kind: "unavailable" };
			}
		},
	});

	const { form: formData, enhance, submitting } = form;

	const words = $derived(copy[intent]);
</script>

<Dialog.Root {open} onOpenChange={(next) => !next && onclose()}>
	<Dialog.Content class="sm:max-w-120">
		<Dialog.Header>
			<Dialog.Title>{words.title}</Dialog.Title>
			<Dialog.Description>{words.description}</Dialog.Description>
		</Dialog.Header>

		{#if check}
			<p class="text-sm leading-normal text-ink-900 text-pretty">{check.statement}</p>
		{/if}

		<form method="POST" id={formId} use:enhance class="flex flex-col gap-5">
			<Form.Field {form} name="reason">
				<Form.Control>
					{#snippet children({ props })}
						<Form.Label>{words.label}</Form.Label>
						<Textarea
							{...props}
							bind:value={$formData.reason}
							disabled={$submitting}
							rows={4}
							placeholder={words.placeholder}
						/>
					{/snippet}
				</Form.Control>
				<Form.FieldErrors />
			</Form.Field>

			{#if failure}
				<p class="text-sm leading-normal text-destructive text-pretty" role="alert">
					{checkFailureMessage(failure)}
				</p>
			{/if}

			<Dialog.Footer>
				<Button
					type="button"
					variant="secondary"
					disabled={$submitting}
					onclick={onclose}
				>
					Cancel
				</Button>
				<Form.Button disabled={$submitting}>
					{$submitting ? words.working : words.submit}
				</Form.Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>

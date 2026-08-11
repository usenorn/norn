<script lang="ts">
	import { defaults, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import { api } from "$lib/api";
	import * as Dialog from "$lib/components/ui/dialog/index.js";
	import * as Form from "$lib/components/ui/form/index.js";
	import * as Select from "$lib/components/ui/select/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { Textarea } from "$lib/components/ui/textarea/index.js";
	import { newCheckSchema } from "./check-schema";
	import {
		checkFailureMessage,
		methodHints,
		methodLabels,
		readCheckFailure,
		type CheckFailure,
		type CheckMethod,
		type IssueCheck,
	} from "./checks";

	let {
		open = $bindable(false),
		workspaceId,
		issueId,
		reference,
		onadded,
	}: {
		open?: boolean;
		workspaceId: string;
		issueId: string;
		reference: string;
		onadded: (checks: IssueCheck[]) => void;
	} = $props();

	const formId = "new-check-form";
	const methods: CheckMethod[] = ["command", "observation", "manual", "regression"];

	let failure = $state<CheckFailure | null>(null);

	const form = superForm(defaults(zod4(newCheckSchema)), {
		id: formId,
		SPA: true,
		validators: zod4Client(newCheckSchema),
		resetForm: true,
		onUpdate: async ({ form: entered }) => {
			if (!entered.valid) return;

			failure = null;

			try {
				const { data, error } = await api.POST(
					"/workspaces/{workspaceId}/issues/{issueId}/checks",
					{
						params: { path: { workspaceId, issueId } },
						body: {
							checks: [
								{
									statement: entered.data.statement,
									method: entered.data.method,
									proof: entered.data.proof,
								},
							],
						},
					}
				);

				if (error) {
					failure = readCheckFailure(error);

					return;
				}

				onadded(data ?? []);
				open = false;
			} catch {
				failure = { kind: "unavailable" };
			}
		},
	});

	const { form: formData, enhance, submitting } = form;

	const method = $derived(($formData.method ?? "command") as CheckMethod);
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="sm:max-w-120">
		<Dialog.Header>
			<Dialog.Title>What has to be true for {reference} to be done?</Dialog.Title>
			<Dialog.Description>
				Write it as a claim somebody could disagree with, and say how it gets proven. Nobody
				should be asked to verify something nothing can produce.
			</Dialog.Description>
		</Dialog.Header>

		<form method="POST" id={formId} use:enhance class="flex flex-col gap-5">
			<Form.Field {form} name="statement">
				<Form.Control>
					{#snippet children({ props })}
						<Form.Label>The claim</Form.Label>
						<Input
							{...props}
							bind:value={$formData.statement}
							disabled={$submitting}
							placeholder="The retry path charges once"
						/>
					{/snippet}
				</Form.Control>
				<Form.FieldErrors />
			</Form.Field>

			<Form.Field {form} name="method">
				<Form.Control>
					{#snippet children({ props })}
						<Form.Label>How it gets proven</Form.Label>
						<Select.Root
							type="single"
							value={$formData.method}
							disabled={$submitting}
							onValueChange={(value) => ($formData.method = value as CheckMethod)}
						>
							<Select.Trigger {...props}>{methodLabels[method]}</Select.Trigger>
							<Select.Content>
								{#each methods as choice (choice)}
									<Select.Item value={choice} label={methodLabels[choice]}>
										{methodLabels[choice]}
									</Select.Item>
								{/each}
							</Select.Content>
						</Select.Root>
					{/snippet}
				</Form.Control>
				<Form.Description class="text-sm text-muted-foreground">
					{methodHints[method]}
				</Form.Description>
				<Form.FieldErrors />
			</Form.Field>

			<Form.Field {form} name="proof">
				<Form.Control>
					{#snippet children({ props })}
						<Form.Label>The path its proof travels</Form.Label>
						<Textarea
							{...props}
							bind:value={$formData.proof}
							disabled={$submitting}
							rows={3}
							placeholder="go test ./internal/service/billing/..."
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
					onclick={() => (open = false)}
				>
					Cancel
				</Button>
				<Form.Button disabled={$submitting}>
					{$submitting ? "Adding…" : "Add it to what done means"}
				</Form.Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>

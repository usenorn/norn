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
	import { evidenceSchema } from "./check-schema";
	import {
		channelLabels,
		checkFailureMessage,
		readCheckFailure,
		verdictLabels,
		type CheckFailure,
		type EvidenceChannel,
		type EvidenceVerdict,
		type IssueCheck,
	} from "./checks";

	let {
		open,
		workspaceId,
		issueId,
		check,
		onfiled,
		onclose,
	}: {
		open: boolean;
		workspaceId: string;
		issueId: string;
		check: IssueCheck | null;
		onfiled: () => void;
		onclose: () => void;
	} = $props();

	const formId = "file-evidence-form";
	const verdicts: EvidenceVerdict[] = ["passed", "failed", "absent_negative", "inconclusive"];
	const channels: EvidenceChannel[] = [
		"command",
		"http",
		"log",
		"screenshot",
		"database",
		"human",
	];

	let failure = $state<CheckFailure | null>(null);

	const form = superForm(defaults(zod4(evidenceSchema)), {
		id: formId,
		SPA: true,
		validators: zod4Client(evidenceSchema),
		resetForm: true,
		onUpdate: async ({ form: entered }) => {
			if (!entered.valid || !check) return;

			failure = null;

			try {
				const { error } = await api.POST(
					"/workspaces/{workspaceId}/issues/{issueId}/checks/{checkId}/evidence",
					{
						params: { path: { workspaceId, issueId, checkId: check.id } },
						body: {
							verdict: entered.data.verdict,
							channel: entered.data.channel,
							command: entered.data.command || undefined,
							output: entered.data.output,
						},
					}
				);

				if (error) {
					failure = readCheckFailure(error);

					return;
				}

				onfiled();
				onclose();
			} catch {
				failure = { kind: "unavailable" };
			}
		},
	});

	const { form: formData, enhance, submitting } = form;

	const verdict = $derived(($formData.verdict ?? "passed") as EvidenceVerdict);
	const channel = $derived(($formData.channel ?? "command") as EvidenceChannel);
</script>

<Dialog.Root {open} onOpenChange={(next) => !next && onclose()}>
	<Dialog.Content class="sm:max-w-140">
		<Dialog.Header>
			<Dialog.Title>File what you saw</Dialog.Title>
			<Dialog.Description>
				{check ? check.statement : "Against this criterion."}
			</Dialog.Description>
		</Dialog.Header>

		{#if check?.method === "manual"}
			<p class="text-sm leading-normal text-muted-foreground text-pretty">
				This one is manual, so only a person's word proves it. What you file here is that word.
			</p>
		{/if}

		<form method="POST" id={formId} use:enhance class="flex flex-col gap-5">
			<div class="flex flex-col gap-5 sm:flex-row sm:gap-4">
				<Form.Field {form} name="verdict" class="min-w-0 flex-1">
					<Form.Control>
						{#snippet children({ props })}
							<Form.Label>What it showed</Form.Label>
							<Select.Root
								type="single"
								value={$formData.verdict}
								disabled={$submitting}
								onValueChange={(value) => ($formData.verdict = value as EvidenceVerdict)}
							>
								<Select.Trigger {...props}>{verdictLabels[verdict]}</Select.Trigger>
								<Select.Content>
									{#each verdicts as choice (choice)}
										<Select.Item value={choice} label={verdictLabels[choice]}>
											{verdictLabels[choice]}
										</Select.Item>
									{/each}
								</Select.Content>
							</Select.Root>
						{/snippet}
					</Form.Control>
					<Form.FieldErrors />
				</Form.Field>

				<Form.Field {form} name="channel" class="min-w-0 flex-1">
					<Form.Control>
						{#snippet children({ props })}
							<Form.Label>Where it came from</Form.Label>
							<Select.Root
								type="single"
								value={$formData.channel}
								disabled={$submitting}
								onValueChange={(value) => ($formData.channel = value as EvidenceChannel)}
							>
								<Select.Trigger {...props}>{channelLabels[channel]}</Select.Trigger>
								<Select.Content>
									{#each channels as choice (choice)}
										<Select.Item value={choice} label={channelLabels[choice]}>
											{channelLabels[choice]}
										</Select.Item>
									{/each}
								</Select.Content>
							</Select.Root>
						{/snippet}
					</Form.Control>
					<Form.FieldErrors />
				</Form.Field>
			</div>

			{#if verdict === "absent_negative"}
				<p class="text-sm leading-normal text-warning text-pretty">
					Absence of a failure never proves a criterion. This will be recorded, and the
					criterion will stay unproven.
				</p>
			{/if}

			<Form.Field {form} name="command">
				<Form.Control>
					{#snippet children({ props })}
						<Form.Label>What you ran</Form.Label>
						<Input
							{...props}
							bind:value={$formData.command}
							disabled={$submitting}
							placeholder="go test ./internal/service/billing/..."
						/>
					{/snippet}
				</Form.Control>
				<Form.Description class="text-sm text-muted-foreground">
					Optional, and exactly as you ran it.
				</Form.Description>
				<Form.FieldErrors />
			</Form.Field>

			<Form.Field {form} name="output">
				<Form.Control>
					{#snippet children({ props })}
						<Form.Label>What it printed</Form.Label>
						<Textarea
							{...props}
							bind:value={$formData.output}
							disabled={$submitting}
							rows={8}
							class="font-mono text-sm"
							placeholder="Paste it verbatim, not a summary of it."
						/>
					{/snippet}
				</Form.Control>
				<Form.Description class="text-sm text-muted-foreground">
					Norn removes anything that looks like a secret before storing it.
				</Form.Description>
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
				<Form.Button disabled={$submitting}>{$submitting ? "Filing…" : "File it"}</Form.Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>

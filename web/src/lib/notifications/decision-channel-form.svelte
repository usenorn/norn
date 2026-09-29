<script lang="ts">
	import { superForm, type SuperValidated } from "sveltekit-superforms";
	import { zod4Client } from "sveltekit-superforms/adapters";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Form from "$lib/components/ui/form/index.js";
	import * as RadioGroup from "$lib/components/ui/radio-group/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import {
		decisionChannelHints,
		decisionChannelLabels,
		decisionChannels,
		type DecisionChannelOutcome,
	} from "./decision-channel";
	import { decisionChannelSchema, type DecisionChannelInput } from "./decision-channel-schema";

	let {
		data,
		workspaceId,
		telegramReady,
	}: {
		data: SuperValidated<DecisionChannelInput, DecisionChannelOutcome>;
		workspaceId: string;
		telegramReady: boolean;
	} = $props();

	// svelte-ignore state_referenced_locally
	const form = superForm(data, {
		id: "decision-channel-form",
		validators: zod4Client(decisionChannelSchema),
		resetForm: false,
	});
	const { form: formData, enhance, submitting, message, tainted } = form;
</script>

<form
	method="POST"
	action="?/decisionChannel"
	use:enhance
	class="flex flex-col gap-3"
	aria-labelledby="decision-channel-heading"
>
	<input type="hidden" name="workspaceId" value={workspaceId} />

	<div class="flex flex-col gap-1">
		<h2 id="decision-channel-heading" class="text-md font-medium tracking-snug text-ink-900">
			Decision requests
		</h2>
		<p class="text-sm leading-normal text-muted-foreground text-pretty">
			When a coding agent needs you to answer a question, approve a plan, or review its changes on
			an issue assigned to you.
		</p>
	</div>

	{#if $message?.kind === "failed"}
		<Alert.Root variant="destructive">
			<CircleX aria-hidden="true" />
			<Alert.Title>That did not save</Alert.Title>
			<Alert.Description>Nothing changed. Wait a moment and try again.</Alert.Description>
		</Alert.Root>
	{/if}

	<Form.Fieldset {form} name="channel">
		<Form.Legend class="sr-only">Where decision requests reach you</Form.Legend>
		<RadioGroup.Root name="channel" bind:value={$formData.channel} disabled={$submitting}>
			{#each decisionChannels as channel (channel)}
				{@const unavailable = channel === "telegram" && !telegramReady}
				<div class="flex items-start gap-2">
					<RadioGroup.Item
						id={`decision-channel-${channel}`}
						value={channel}
						class="mt-0.5"
						disabled={unavailable}
					/>
					<label for={`decision-channel-${channel}`} class="flex flex-col gap-0.5">
						<span class="text-sm leading-normal text-ink-900">{decisionChannelLabels[channel]}</span>
						<span class="text-xs leading-normal text-muted-foreground">
							{unavailable
								? "Link your Telegram to an agent's bot below first."
								: decisionChannelHints[channel]}
						</span>
					</label>
				</div>
			{/each}
		</RadioGroup.Root>
		<Form.FieldErrors />
	</Form.Fieldset>

	<div class="flex items-center gap-2">
		<Button type="submit" size="sm" disabled={$submitting || !$tainted}>
			{$submitting ? "Saving…" : "Save"}
		</Button>
		<span role="status" class="text-sm text-muted-foreground">
			{$message?.kind === "saved" && !$tainted ? "Saved." : ""}
		</span>
	</div>
</form>

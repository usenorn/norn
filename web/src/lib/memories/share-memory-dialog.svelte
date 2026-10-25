<script lang="ts">
	import CircleX from "@lucide/svelte/icons/circle-x";
	import EyeOff from "@lucide/svelte/icons/eye-off";
	import Hourglass from "@lucide/svelte/icons/hourglass";
	import { defaults, setError, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import { unknownLine } from "#lib/api/attempt.js";
	import * as Alert from "#lib/components/ui/alert/index.js";
	import { Checkbox } from "#lib/components/ui/checkbox/index.js";
	import * as Dialog from "#lib/components/ui/dialog/index.js";
	import * as Form from "#lib/components/ui/form/index.js";
	import { Button } from "#lib/components/ui/button/index.js";
	import AudienceFields from "#lib/docs/audience-fields.svelte";
	import { audienceLine, sameAudience, type DocAudience } from "#lib/docs/docs.js";
	import type { Project } from "#lib/projects/projects.js";
	import { showToast } from "#lib/toast/toasts.js";
	import { memoryFailureMessage, readMemoryFailure, type Memory } from "./memories";
	import { shareMemoryFormId, shareMemorySchema } from "./memory-schema";
	import { refreshMemory, shareMemory } from "./memory-writes";

	let {
		open = $bindable(false),
		memory,
		projects,
		workspaceId,
		workspaceName,
	}: {
		open?: boolean;
		memory: Memory;
		projects: Project[];
		workspaceId: string;
		workspaceName: string;
	} = $props();

	let failure = $state("");
	let widening = $state(false);

	const form = superForm(defaults(zod4(shareMemorySchema)), {
		id: shareMemoryFormId,
		SPA: true,
		validators: zod4Client(shareMemorySchema),
		resetForm: false,
		invalidateAll: false,
		onSubmit: () => {
			failure = "";
		},
		onUpdate: async ({ form: entered }) => {
			if (!entered.valid) return;

			const audience: DocAudience =
				entered.data.visibility === "project"
					? { visibility: "project", projectId: entered.data.projectId }
					: { visibility: entered.data.visibility };

			if (sameAudience(audience, memory.audience) && entered.data.redacted === memory.redacted) {
				open = false;

				return;
			}

			const outcome = await shareMemory(workspaceId, memory, audience, entered.data.redacted);

			if (outcome.kind === "done") {
				open = false;
				showToast("The memory is shared differently now", {
					detail: audienceLine(audience, projects, workspaceName),
				});
				await refreshMemory(workspaceId, memory.id);

				return;
			}

			if (outcome.kind === "unknown") {
				failure = unknownLine;

				return;
			}

			const read = readMemoryFailure(outcome.problem);

			if (read.kind === "narrower_source" && !widening) {
				widening = true;

				return;
			}

			if (read.kind === "invalid" && entered.data.visibility === "project") {
				setError(entered, "projectId", "Choose a project you can reach.");

				return;
			}

			failure = memoryFailureMessage(read);
		},
	});

	const { form: formData, enhance, submitting } = form;

	const backToReview = $derived(
		memory.status === "accepted" && !memory.viewer.canReview && $formData.visibility !== "personal"
	);

	$effect(() => {
		if (!open) return;

		failure = "";
		widening = memory.redacted;
		$formData = {
			visibility: memory.audience.visibility,
			projectId: memory.audience.projectId ?? "",
			redacted: memory.redacted,
		};
	});
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title>Share this memory differently</Dialog.Title>
			<Dialog.Description>
				Who reads a memory decides which agents recall it. {audienceLine(memory.audience, projects, workspaceName)}
			</Dialog.Description>
		</Dialog.Header>

		<form method="POST" use:enhance class="flex flex-col gap-4">
			{#if failure}
				<Alert.Root variant="destructive">
					<CircleX aria-hidden="true" />
					<Alert.Title>It was not shared differently</Alert.Title>
					<Alert.Description>{failure}</Alert.Description>
				</Alert.Root>
			{/if}

			{#if backToReview}
				<Alert.Root>
					<Hourglass aria-hidden="true" />
					<Alert.Title>It goes back to review</Alert.Title>
					<Alert.Description>
						Agents stop recalling it until a workspace admin, or the lead of its project, accepts it
						again.
					</Alert.Description>
				</Alert.Root>
			{/if}

			<AudienceFields {form} {projects} disabled={$submitting} />

			{#if widening}
				<Alert.Root variant="warning">
					<EyeOff aria-hidden="true" />
					<Alert.Title>A doc it cites is shared with fewer people</Alert.Title>
					<Alert.Description>
						Sharing it this way passes on what the memory says, never the doc. Readers outside the
						doc's audience see that source as unavailable.
					</Alert.Description>
				</Alert.Root>

				<Form.Field {form} name="redacted">
					<Form.Control>
						{#snippet children({ props })}
							<div class="flex items-start gap-2">
								<Checkbox {...props} disabled={$submitting} bind:checked={$formData.redacted} />
								<div class="flex flex-col gap-0.5">
									<Form.Label>Share a redacted statement</Form.Label>
									<span class="text-sm leading-normal text-pretty text-muted-foreground">
										You choose to share the statement wider than its sources.
									</span>
								</div>
							</div>
						{/snippet}
					</Form.Control>
				</Form.Field>
			{/if}

			<Dialog.Footer>
				<Button type="button" variant="ghost" disabled={$submitting} onclick={() => (open = false)}>
					Cancel
				</Button>
				<Button type="submit" disabled={$submitting}>
					{$submitting ? "Sharing" : "Share"}
				</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>

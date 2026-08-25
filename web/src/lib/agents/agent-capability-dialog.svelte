<script lang="ts">
	import { defaults, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import * as Dialog from "$lib/components/ui/dialog/index.js";
	import * as Form from "$lib/components/ui/form/index.js";
	import * as RadioGroup from "$lib/components/ui/radio-group/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { Textarea } from "$lib/components/ui/textarea/index.js";
	import {
		agentCapabilityDraftSchema,
		capabilityDraft,
		type AgentCapabilityDraft,
		type AgentCapabilityDraftInput,
		type AgentCapabilityKind,
	} from "./agent-capabilities";

	let {
		open = $bindable(false),
		kind,
		onadd,
	}: {
		open?: boolean;
		kind: AgentCapabilityKind;
		onadd: (draft: AgentCapabilityDraft) => void;
	} = $props();

	const formId = "agent-capability-draft-form";

	const form = superForm(defaults(zod4(agentCapabilityDraftSchema)), {
		id: formId,
		SPA: true,
		validators: zod4Client(agentCapabilityDraftSchema),
		resetForm: false,
		onUpdate: ({ form: submitted }) => {
			if (!submitted.valid) return;

			onadd(capabilityDraft(submitted.data));
			resetDraft(kind);
			open = false;
		},
	});

	const { form: formData, enhance, submitting } = form;

	const title = $derived(kind === "skill" ? "Add a draft skill" : "Add a draft MCP server");
	const action = $derived(kind === "skill" ? "Add skill" : "Add MCP server");

	$effect(() => {
		if (open) resetDraft(kind);
	});

	function emptyDraft(nextKind: AgentCapabilityKind): AgentCapabilityDraftInput {
		return {
			kind: nextKind,
			name: "",
			source: "",
			transport: "remote",
			command: "",
			args: "",
			url: "",
			auth: "none",
			secret: "",
		};
	}

	function resetDraft(nextKind: AgentCapabilityKind) {
		form.reset({ keepMessage: false });
		$formData = emptyDraft(nextKind);
	}

	function close() {
		resetDraft(kind);
		open = false;
	}
</script>

<Dialog.Root {open} onOpenChange={(next) => (next ? (open = true) : close())}>
	<Dialog.Content variant="scrollable" class="sm:max-w-120">
		<Dialog.Header>
			<Dialog.Title>{title}</Dialog.Title>
			<Dialog.Description>
				This is a development preview. It stays in this browser session and is not sent to the
				agent runtime.
			</Dialog.Description>
		</Dialog.Header>

		<form method="POST" id={formId} use:enhance class="flex flex-col gap-5">
			<input type="hidden" name="kind" value={$formData.kind} />

			{#if kind === "skill"}
				<Form.Field {form} name="name">
					<Form.Control>
						{#snippet children({ props })}
							<Form.Label>Name</Form.Label>
							<Input
								{...props}
								bind:value={$formData.name}
								disabled={$submitting}
								placeholder="Issue triage"
								autocomplete="off"
							/>
						{/snippet}
					</Form.Control>
					<Form.FieldErrors />
				</Form.Field>

				<Form.Field {form} name="source">
					<Form.Control>
						{#snippet children({ props })}
							<Form.Label>Source URL or repository path</Form.Label>
							<Input
								{...props}
								bind:value={$formData.source}
								disabled={$submitting}
								placeholder="https://github.com/nornhq/skills/tree/main/triage"
								autocomplete="url"
							/>
						{/snippet}
					</Form.Control>
					<Form.FieldErrors />
				</Form.Field>
			{:else}
				<Form.Field {form} name="name">
					<Form.Control>
						{#snippet children({ props })}
							<Form.Label>Name</Form.Label>
							<Input
								{...props}
								bind:value={$formData.name}
								disabled={$submitting}
								placeholder="Documentation"
								autocomplete="off"
							/>
						{/snippet}
					</Form.Control>
					<Form.FieldErrors />
				</Form.Field>

				<Form.Fieldset {form} name="transport">
					<Form.Legend>Transport</Form.Legend>
					<RadioGroup.Root name="transport" bind:value={$formData.transport} disabled={$submitting}>
						<div class="flex items-center gap-2">
							<RadioGroup.Item id="agent-mcp-transport-remote" value="remote" />
							<label for="agent-mcp-transport-remote" class="text-sm text-ink-600">
								Remote URL
							</label>
						</div>
						<div class="flex items-center gap-2">
							<RadioGroup.Item id="agent-mcp-transport-stdio" value="stdio" />
							<label for="agent-mcp-transport-stdio" class="text-sm text-ink-600">
								Local command (stdio)
							</label>
						</div>
					</RadioGroup.Root>
					<Form.FieldErrors />
				</Form.Fieldset>

				{#if $formData.transport === "stdio"}
					<Form.Field {form} name="command">
						<Form.Control>
							{#snippet children({ props })}
								<Form.Label>Command</Form.Label>
								<Input
									{...props}
									bind:value={$formData.command}
									disabled={$submitting}
									placeholder="npx"
									autocomplete="off"
								/>
							{/snippet}
						</Form.Control>
						<Form.FieldErrors />
					</Form.Field>

					<Form.Field {form} name="args">
						<Form.Control>
							{#snippet children({ props })}
								<Form.Label>Arguments</Form.Label>
								<Textarea
									{...props}
									bind:value={$formData.args}
									disabled={$submitting}
									placeholder="@playwright/mcp@latest"
								/>
								<Form.Description>Optional. Put each argument on its own line.</Form.Description>
							{/snippet}
						</Form.Control>
						<Form.FieldErrors />
					</Form.Field>
				{:else}
					<Form.Field {form} name="url">
						<Form.Control>
							{#snippet children({ props })}
								<Form.Label>Server URL</Form.Label>
								<Input
									{...props}
									bind:value={$formData.url}
									disabled={$submitting}
									placeholder="https://mcp.example.com"
									autocomplete="url"
								/>
								<Form.Description>Supports Streamable HTTP and legacy SSE servers.</Form.Description>
							{/snippet}
						</Form.Control>
						<Form.FieldErrors />
					</Form.Field>

					<Form.Fieldset {form} name="auth">
						<Form.Legend>Authentication</Form.Legend>
						<RadioGroup.Root name="auth" bind:value={$formData.auth} disabled={$submitting}>
							<div class="flex items-center gap-2">
								<RadioGroup.Item id="agent-mcp-auth-none" value="none" />
								<label for="agent-mcp-auth-none" class="text-sm text-ink-600">None</label>
							</div>
							<div class="flex items-center gap-2">
								<RadioGroup.Item id="agent-mcp-auth-bearer" value="bearer" />
								<label for="agent-mcp-auth-bearer" class="text-sm text-ink-600">Bearer token</label>
							</div>
						</RadioGroup.Root>
						<Form.FieldErrors />
					</Form.Fieldset>

					{#if $formData.auth === "bearer"}
						<Form.Field {form} name="secret">
							<Form.Control>
								{#snippet children({ props })}
									<Form.Label>Bearer token</Form.Label>
									<Input
										{...props}
										type="password"
										bind:value={$formData.secret}
										disabled={$submitting}
										autocomplete="new-password"
									/>
								{/snippet}
							</Form.Control>
							<Form.Description>The token is cleared when this dialog closes and is never retained.</Form.Description>
							<Form.FieldErrors />
						</Form.Field>
					{/if}
				{/if}
			{/if}

			<Dialog.Footer>
				<Button type="button" variant="secondary" disabled={$submitting} onclick={close}>Cancel</Button>
				<Form.Button disabled={$submitting}>{action}</Form.Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>

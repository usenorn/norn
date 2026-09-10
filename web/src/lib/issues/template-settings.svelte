<script lang="ts">
	import CircleX from "@lucide/svelte/icons/circle-x";
	import Plus from "@lucide/svelte/icons/plus";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Dialog from "$lib/components/ui/dialog/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Checkbox } from "$lib/components/ui/checkbox/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { Label } from "$lib/components/ui/label/index.js";
	import Editor from "$lib/editor/editor.svelte";
	import { asDocument, emptyDocument, type Document } from "$lib/editor/document";
	import {
		readTemplates,
		removeTemplate,
		saveTemplate,
		templateFailureMessage,
		templateFields,
		templatesFor,
		templatesOf,
		type IssueTemplate,
		type TemplateFailure,
		type TemplateField,
		type TemplateList,
	} from "$lib/issues/templates";

	let {
		workspaceId,
		workspace,
		teamId,
		teamName,
		locked = false,
	}: {
		workspaceId: string;
		workspace: string;
		teamId: string;
		teamName: string;
		locked?: boolean;
	} = $props();

	let list = $state.raw<TemplateList>({ kind: "loading" });
	let failure = $state.raw<TemplateFailure | null>(null);
	let editing = $state(false);
	let working = $state(false);

	let templateId = $state("");
	let name = $state("");
	let description = $state("");
	let title = $state("");
	let body = $state.raw<Document>(emptyDocument);
	let required = $state.raw<TemplateField[]>([]);

	const templates = $derived(templatesOf(list).filter((template) => template.teamId === teamId));

	$effect(() => {
		void refresh();
	});

	async function refresh() {
		list = templatesFor(await readTemplates(workspaceId, teamId));
	}

	function start(template?: IssueTemplate) {
		templateId = template?.id ?? "";
		name = template?.name ?? "";
		description = template?.description ?? "";
		title = template?.title ?? "";
		body = asDocument(template?.bodyDoc ?? emptyDocument);
		required = template?.requiredFields ?? [];
		failure = null;
		editing = true;
	}

	function insist(field: TemplateField, on: boolean) {
		required = on ? [...required, field] : required.filter((held) => held !== field);
	}

	async function keep() {
		if (name.trim() === "") return;

		working = true;
		failure = null;

		const outcome = await saveTemplate(workspaceId, {
			templateId: templateId || undefined,
			teamId,
			name: name.trim(),
			description,
			title,
			body,
			requiredFields: required,
		});

		working = false;

		if (outcome.failure) {
			failure = outcome.failure;

			return;
		}

		editing = false;
		await refresh();
	}

	async function forget(template: IssueTemplate) {
		working = true;
		failure = null;

		if (!(await removeTemplate(workspaceId, template.id))) {
			failure = { kind: "unavailable" };
		}

		working = false;
		await refresh();
	}
</script>

<section class="flex flex-col gap-4">
	<div class="flex flex-col gap-1">
		<h2 class="text-md font-medium tracking-snug text-ink-900">Issue templates</h2>
		<p class="text-sm leading-normal text-muted-foreground text-pretty">
			The shapes {teamName} raises issues from: the prose an issue starts with, and the properties
			it insists on before it may be raised.
		</p>
	</div>

	{#if failure}
		<Alert.Root variant="destructive">
			<CircleX aria-hidden="true" />
			<Alert.Title>That did not work</Alert.Title>
			<Alert.Description>{templateFailureMessage(failure)}</Alert.Description>
		</Alert.Root>
	{/if}

	{#if list.kind === "loading"}
		<div class="h-16 animate-breathe rounded-lg bg-paper-2" aria-busy="true"></div>
	{:else if list.kind === "unavailable"}
		<p class="text-md text-muted-foreground">We could not read this team's templates.</p>
	{:else if templates.length === 0}
		<p class="text-md text-muted-foreground">
			This team keeps no templates. An issue starts from an empty description.
		</p>
	{:else}
		<ul class="flex flex-col rounded-md border border-line-default">
			{#each templates as template (template.id)}
				<li
					class="flex items-center gap-2 border-b border-line-subtle px-2.75 py-2 last:border-b-0"
				>
					<span class="flex min-w-0 flex-1 flex-col">
						<span class="truncate text-md text-ink-900">{template.name}</span>
						{#if template.description}
							<span class="truncate text-sm text-muted-foreground">{template.description}</span>
						{/if}
						{#if template.requiredFields.length > 0}
							<span class="text-xs text-muted-foreground">
								Insists on {template.requiredFields.join(", ")}
							</span>
						{/if}
					</span>
					<Button variant="ghost" size="sm" disabled={locked || working} onclick={() => start(template)}>
						Edit
					</Button>
					<Button variant="ghost" size="sm" disabled={locked || working} onclick={() => forget(template)}>
						Remove
					</Button>
				</li>
			{/each}
		</ul>
	{/if}

	<div>
		<Button variant="secondary" disabled={locked || working} onclick={() => start()}>
			<Plus aria-hidden="true" />
			Add a template
		</Button>
	</div>
</section>

<Dialog.Root bind:open={editing}>
	<Dialog.Content class="max-h-[calc(100dvh-6rem)] overflow-y-auto sm:max-w-2xl">
		<Dialog.Header>
			<Dialog.Title>{templateId ? "Change this template" : "A new template"}</Dialog.Title>
			<Dialog.Description>
				What an issue raised from this starts with. Anything the person raising it chooses wins
				over what the template offers.
			</Dialog.Description>
		</Dialog.Header>

		<div class="flex flex-col gap-3.5">
			<div class="flex flex-col gap-1.5">
				<Label for="template-name">Name</Label>
				<Input id="template-name" bind:value={name} placeholder="Bug report" disabled={working} />
			</div>

			<div class="flex flex-col gap-1.5">
				<Label for="template-description">What it is for</Label>
				<Input
					id="template-description"
					bind:value={description}
					placeholder="Shown beside the name when choosing one"
					disabled={working}
				/>
			</div>

			<div class="flex flex-col gap-1.5">
				<Label for="template-title">The title it starts with</Label>
				<Input
					id="template-title"
					bind:value={title}
					placeholder="Left empty, the person writes their own"
					disabled={working}
				/>
			</div>

			<div class="flex flex-col gap-1.5">
				<span class="text-sm font-medium text-ink-900">The description it starts with</span>
				<div class="rounded-md border border-line-default px-2.5 pb-1">
					<Editor
						bind:document={body}
						{workspaceId}
						{workspace}
						disabled={working}
						placeholder="The headings and prompts an issue of this kind starts with"
						label="The description this template starts with"
					/>
				</div>
			</div>

			<fieldset class="flex flex-col gap-1.5">
				<legend class="text-sm font-medium text-ink-900">What it insists on</legend>
				<p class="text-sm text-muted-foreground">
					An issue raised from this template is refused until these are chosen.
				</p>
				<div class="flex flex-wrap gap-x-4 gap-y-1.5 pt-1">
					{#each templateFields as choice (choice.field)}
						<div class="flex items-center gap-1.5">
							<Checkbox
								id="template-requires-{choice.field}"
								checked={required.includes(choice.field)}
								disabled={working}
								onCheckedChange={(on) => insist(choice.field, on === true)}
							/>
							<Label for="template-requires-{choice.field}" class="font-normal">
								{choice.label}
							</Label>
						</div>
					{/each}
				</div>
			</fieldset>
		</div>

		<Dialog.Footer>
			<Button variant="ghost" disabled={working} onclick={() => (editing = false)}>Cancel</Button>
			<Button disabled={working || name.trim() === ""} onclick={keep}>
				{working ? "Saving" : "Save template"}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

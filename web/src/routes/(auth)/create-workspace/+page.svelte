<script lang="ts">
	import { goto } from "$app/navigation";
	import { page } from "$app/state";
	import { defaults, setError, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import * as Form from "$lib/components/ui/form/index.js";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import StepList, { type Step } from "$lib/components/norn/step-list.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { Progress } from "$lib/components/ui/progress/index.js";
	import { api } from "$lib/api";
	import {
		createWorkspaceSchema,
		slugMessage,
		slugSuggestions,
		slugFromName,
	} from "$lib/workspace/create-workspace-schema";
	import { teamKeyFromName, teamKeyMessage, teamKeySuggestions } from "$lib/team/teams";
	import type { WorkspaceCreationFailure } from "$lib/workspace/types";
	import { createWorkspacePreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV
			? createWorkspacePreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined
	);

	let submitFailure = $state<WorkspaceCreationFailure | null>(null);
	let slugEdited = $state(false);
	let keyEdited = $state(false);

	const auth = $derived(data.auth);
	const workspace = $derived({ ...data.workspace, ...preview?.workspace });
	const failure = $derived<WorkspaceCreationFailure | null>(preview?.failure ?? submitFailure);

	const form = superForm(defaults(zod4(createWorkspaceSchema)), {
		SPA: true,
		validators: zod4Client(createWorkspaceSchema),
		resetForm: false,
		onUpdate: async ({ form: submitted }) => {
			if (!submitted.valid) return;

			submitFailure = null;

			try {
				const { data: created, error } = await api.POST("/workspaces", {
					body: {
						slug: submitted.data.slug,
						name: submitted.data.name,
						team: { key: submitted.data.teamKey, name: submitted.data.teamName },
					},
				});

				if (created) {
					await goto(`/invite-teammates?workspace=${created.slug}`);

					return;
				}

				if (error && "code" in error && error.code === "team_key_taken") {
					submitFailure = {
						kind: "team_key_taken",
						key: submitted.data.teamKey,
						suggestions: teamKeySuggestions(submitted.data.teamName, submitted.data.teamKey, []),
					};

					return;
				}

				if (error?.status === 409) {
					submitFailure = {
						kind: "slug_taken",
						slug: submitted.data.slug,
						suggestions: slugSuggestions(submitted.data.slug),
					};

					return;
				}

				for (const field of error?.errors ?? []) {
					if (field.field === "slug") setError(submitted, "slug", slugMessage(field.code));
					if (field.field === "name") setError(submitted, "name", "Enter a workspace name.");
					if (field.field === "key") setError(submitted, "teamKey", teamKeyMessage(field.code));
				}
			} catch {
				setError(submitted, "name", "We could not reach the server. Try again in a moment.");
			}
		},
	});
	const { form: formData, enhance, submitting } = form;

	$effect(() => {
		const prefill = preview?.form;
		if (prefill) formData.update((current) => ({ ...current, ...prefill }), { taint: false });
	});

	const busy = $derived(preview?.busy || $submitting);
	const additional = $derived(workspace.existingWorkspace !== null);
	const slugTaken = $derived(failure?.kind === "slug_taken" && failure.slug === $formData.slug);
	const keyTaken = $derived(
		failure?.kind === "team_key_taken" && failure.key === $formData.teamKey.toUpperCase()
	);

	const eyebrow = $derived(additional ? "Second workspace" : "Step 3 of 4");
	const title = $derived(additional ? "Create another workspace" : "Create a workspace");
	const lede = $derived(
		additional
			? `A second workspace shares nothing with ${workspace.existingWorkspace} — separate members, projects and history.`
			: "Two things now. Everything else is a setting."
	);

	const steps = $derived<Step[]>([
		{ label: "Workspace created", state: "done" },
		{
			label: $formData.teamName
				? `Setting up the ${$formData.teamName} team`
				: "Setting up your first team",
			state: "active",
		},
		{ label: "Preparing your first cycle", state: "waiting" },
	]);

	const footer = $derived(
		additional
			? "Switch workspaces from the sidebar, or with ⌘⇧O."
			: "The names can change later. The address and the team key are permanent, because they appear in every issue reference."
	);

	function pickSuggestion(slug: string) {
		slugEdited = true;
		formData.update((current) => ({ ...current, slug }));
		submitFailure = null;
	}

	function pickKey(teamKey: string) {
		keyEdited = true;
		formData.update((current) => ({ ...current, teamKey }));
		submitFailure = null;
	}

	function fieldValue(event: Event) {
		return (event.currentTarget as HTMLInputElement).value;
	}

	function deriveSlug(event: Event) {
		if (slugEdited) return;
		$formData.slug = slugFromName(fieldValue(event));
	}

	function deriveTeamKey(event: Event) {
		if (keyEdited) return;
		$formData.teamKey = teamKeyFromName(fieldValue(event));
	}

	function claimSlug(event: Event) {
		slugEdited = fieldValue(event).trim() !== "";
	}

	function claimKey(event: Event) {
		keyEdited = fieldValue(event).trim() !== "";
	}
</script>

<svelte:head><title>{title} · Norn</title></svelte:head>

<div class="my-auto flex w-full flex-col items-center gap-4">
	<div class="notch w-full max-w-105">
		<div class="flex flex-col gap-5 p-5 sm:p-6">
			<div class="flex flex-col gap-1.5">
				<Eyebrow>{eyebrow}</Eyebrow>
				<h1 class="text-2xl font-medium tracking-title text-ink-900">{title}</h1>
				<p class="text-md leading-normal text-muted-foreground text-pretty">{lede}</p>
			</div>

			{#if !busy}
				<form
					id="create-workspace-form"
					method="POST"
					use:enhance
					class="flex flex-col gap-4"
				>
					<Form.Field {form} name="name">
						<Form.Control>
							{#snippet children({ props })}
								<Form.Label>Workspace name</Form.Label>
								<Input
									{...props}
									autocomplete="organization"
									oninput={deriveSlug}
									bind:value={$formData.name}
								/>
							{/snippet}
						</Form.Control>
						<Form.FieldErrors />
					</Form.Field>

					<Form.Field {form} name="slug">
						<Form.Control>
							{#snippet children({ props })}
								<Form.Label>Address</Form.Label>
								<div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:gap-0.5">
									<span
										class="font-mono text-md break-all text-muted-foreground sm:whitespace-nowrap"
									>
										{auth.host}/
									</span>
									<Input
										{...props}
										autocapitalize="none"
										spellcheck="false"
										aria-invalid={slugTaken ? "true" : undefined}
										oninput={claimSlug}
										bind:value={$formData.slug}
									/>
								</div>
							{/snippet}
						</Form.Control>
						{#if slugTaken && failure?.kind === "slug_taken"}
							<p class="flex items-center gap-1.5 text-sm text-destructive" role="alert">
								<CircleAlert class="size-icon-row shrink-0" aria-hidden="true" />
								{failure.slug} is taken on this instance.
							</p>
						{:else}
							<Form.Description class="text-sm text-muted-foreground">
								Used in links and mentions.
							</Form.Description>
						{/if}
						<Form.FieldErrors />
					</Form.Field>

					{#if slugTaken && failure?.kind === "slug_taken"}
						<div class="flex flex-wrap items-center gap-1.5">
							<span class="text-sm text-muted-foreground">Free:</span>
							{#each failure.suggestions as suggestion (suggestion)}
								<Button variant="chip" size="chip" onclick={() => pickSuggestion(suggestion)}>
									{suggestion}
								</Button>
							{/each}
						</div>
					{/if}

					<span class="h-px bg-line-subtle" aria-hidden="true"></span>

					<div class="flex flex-wrap gap-3">
						<div class="min-w-[150px] flex-[1_1_180px]">
							<Form.Field {form} name="teamName">
								<Form.Control>
									{#snippet children({ props })}
										<Form.Label>First team</Form.Label>
										<Input
											{...props}
											disabled={busy}
											oninput={deriveTeamKey}
											bind:value={$formData.teamName}
										/>
									{/snippet}
								</Form.Control>
								<Form.FieldErrors />
							</Form.Field>
						</div>
						<div class="min-w-20 flex-[0_1_92px]">
							<Form.Field {form} name="teamKey">
								<Form.Control>
									{#snippet children({ props })}
										<Form.Label>Key</Form.Label>
										<Input
											{...props}
											maxlength={5}
											autocapitalize="characters"
											spellcheck="false"
											disabled={busy}
											aria-invalid={keyTaken ? "true" : undefined}
											class="uppercase"
											oninput={claimKey}
											bind:value={$formData.teamKey}
										/>
									{/snippet}
								</Form.Control>
								<Form.FieldErrors />
							</Form.Field>
						</div>
					</div>

					{#if keyTaken && failure?.kind === "team_key_taken"}
						<div class="flex flex-col gap-1.5">
							<p class="flex items-center gap-1.5 text-sm text-destructive" role="alert">
								<CircleAlert class="size-icon-row shrink-0" aria-hidden="true" />
								{failure.key} is already used in this workspace.
							</p>
							{#if failure.suggestions.length > 0}
								<div class="flex flex-wrap items-center gap-1.5">
									<span class="text-sm text-muted-foreground">Free:</span>
									{#each failure.suggestions as suggestion (suggestion)}
										<Button
											variant="chip"
											size="chip"
											onclick={() => pickKey(suggestion)}
										>
											{suggestion}
										</Button>
									{/each}
								</div>
							{/if}
						</div>
					{:else}
						<p class="text-sm leading-normal text-muted-foreground">
							Issues in this team are numbered
							<span class="font-mono text-ink-600">{$formData.teamKey || "MOB"}-1</span>,
							<span class="font-mono text-ink-600">{$formData.teamKey || "MOB"}-2</span>, and so on.
							The key is permanent.
						</p>
					{/if}
				</form>
			{:else}
				<div class="flex flex-col gap-2" aria-live="polite">
					<Progress indeterminate aria-label="Creating workspace" />
					<StepList {steps} />
				</div>
			{/if}

			<Button type="submit" form="create-workspace-form" class="w-full" disabled={busy}>
				{busy ? "Creating workspace" : "Create workspace"}
			</Button>
		</div>
	</div>

	<p class="max-w-100 text-center text-sm leading-normal text-muted-foreground text-pretty">
		{footer}
	</p>
</div>

<script lang="ts">
	import { enhance as changeEnhance } from "$app/forms";
	import { goto, invalidateAll } from "$app/navigation";
	import { page } from "$app/state";
	import { superForm } from "sveltekit-superforms";
	import { zod4Client } from "sveltekit-superforms/adapters";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import Settings from "@lucide/svelte/icons/settings";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as AlertDialog from "$lib/components/ui/alert-dialog/index.js";
	import * as Form from "$lib/components/ui/form/index.js";
	import * as Select from "$lib/components/ui/select/index.js";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import TeamKey from "$lib/components/norn/team-key.svelte";
	import WorkspaceMark from "$lib/components/norn/workspace-mark.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { Textarea } from "$lib/components/ui/textarea/index.js";
	import { Progress } from "$lib/components/ui/progress/index.js";
	import { withSlot } from "$lib/account/accounts";
	import SaveBar from "$lib/settings/save-bar.svelte";
	import SettingRow from "$lib/settings/setting-row.svelte";
	import SettingsPage from "$lib/settings/settings-page.svelte";
	import { showToast } from "$lib/toast/toasts";
	import {
		logoAccept,
		logoFailureMessage,
		logoSource,
		removeLogo,
		uploadLogo,
		type LogoActivity,
	} from "$lib/workspace/logo";
	import { workspacePath } from "$lib/workspace/navigation";
	import { workspaceSettingsSchema, workspaceSlugPattern } from "$lib/workspace/settings-schema";
	import {
		purgeDate,
		redirectEnds,
		saveBarOf,
		slugRedirectDays,
		timezones,
		weekDays,
		type WeekDay,
		type Workspace,
		type WorkspaceSettings,
	} from "$lib/workspace/settings";
	import {
		headroomLabel,
		headroomToneClass,
		measuredLabel,
		storageToneOf,
		storedLabel,
	} from "$lib/workspace/storage";
	import { workspaceSettingsPreviewStates } from "./preview";
	import type { PageProps, SubmitFunction } from "./$types";

	const formId = "workspace-settings-form";

	let { data, form: submitted }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV
			? workspaceSettingsPreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined
	);

	let confirmation = $state("");
	let changing = $state(false);
	let confirmingDelete = $state(false);
	let logo = $state<LogoActivity>({ kind: "idle" });
	let logoInput = $state<HTMLInputElement | null>(null);

	function formValues(workspace: Workspace) {
		return {
			name: workspace.name,
			slug: workspace.slug,
			timezone: workspace.timezone,
			weekStartsOn: workspace.weekStartsOn,
			agentInstructions: workspace.agentInstructions ?? "",
			defaultTeamId: workspace.defaultTeamId ?? "",
		};
	}

	// svelte-ignore state_referenced_locally
	const form = superForm(data.form, {
		id: formId,
		validators: zod4Client(workspaceSettingsSchema),
		resetForm: false,
		invalidateAll: false,
		onUpdated: async ({ form: updated }) => {
			const outcome = updated.message;

			if (outcome?.kind !== "saved") return;

			const renamed = outcome.workspace.name !== data.workspace.name;

			if (outcome.renamedFrom) {
				await goto(withSlot(workspacePath(outcome.workspace.slug, "/settings"), data.member.slot), {
					replaceState: true,
					invalidateAll: true,
				});
			} else {
				await invalidateAll();
			}

			showToast(renamed ? `Saved. Renamed to ${outcome.workspace.name}` : "Saved.");
		},
	});
	const { form: formData, enhance, submitting, message, errors, tainted, isTainted, reset } = form;

	const settings = $derived<WorkspaceSettings>(
		submitted?.settings ?? $message ?? preview?.settings ?? data.settings
	);
	const workspace = $derived("workspace" in settings ? settings.workspace : data.workspace);
	const pending = $derived(settings.kind === "pending_deletion" ? settings : null);
	const teams = $derived(preview?.teams ?? data.teams);
	const storage = $derived(preview?.storage ?? data.storage);
	const storageTone = $derived(storageToneOf(storage));
	const headroomClass = $derived(
		`text-sm leading-normal text-pretty ${headroomToneClass[storageTone]}`
	);
	const zones = timezones();

	const role = $derived(
		preview?.role ?? data.members.find((member) => member.accountId === data.member.id)?.role
	);
	const administrator = $derived(role === "admin");
	const locked = $derived(!administrator || pending !== null);

	$effect(() => {
		if (preview?.draft) return;

		formData.update(() => formValues(workspace), { taint: false });
	});

	$effect(() => {
		if (preview?.confirmingDelete) confirmingDelete = true;
		if (preview?.logo) logo = preview.logo;
		if (preview?.settings.kind === "saved") showToast(`Saved. Renamed to ${preview.settings.workspace.name}`);
	});

	const busy = $derived($submitting || changing || preview?.saving === true);
	const dirty = $derived(isTainted($tainted) || preview?.draft !== undefined);
	const bar = $derived(
		saveBarOf(dirty && !locked, $submitting || preview?.saving === true, ($errors.slug?.length ?? 0) > 0)
	);

	const savedSlug = $derived(workspace.slug);
	const draftSlug = $derived($formData.slug.trim().toLowerCase());
	const movingAddress = $derived(
		draftSlug !== savedSlug &&
			draftSlug.length > 1 &&
			workspaceSlugPattern.test(draftSlug) &&
			($errors.slug?.length ?? 0) === 0
	);
	const host = $derived(page.url.host);
	const exampleKey = $derived(teams[0]?.key);
	const logoUrl = $derived(preview?.logoUrl ?? logoSource(workspace.logoUrl, data.member.slot));
	const logoBusy = $derived(logo.kind === "uploading" || logo.kind === "removing");
	const confirmed = $derived(confirmation.trim().toLowerCase() === savedSlug);

	const trackChange: SubmitFunction = () => {
		changing = true;

		return async ({ result, update }) => {
			await update();

			if (result.type === "success") {
				confirmation = "";
				confirmingDelete = false;
				reset({ data: formValues(data.workspace) });
			}

			changing = false;
		};
	};

	function discard() {
		reset({ data: formValues(workspace) });
	}

	async function replaceLogo(event: Event & { currentTarget: HTMLInputElement }) {
		const file = event.currentTarget.files?.[0];
		event.currentTarget.value = "";

		if (!file) return;

		logo = { kind: "uploading" };

		const outcome = await uploadLogo(workspace.id, file);

		if (outcome.kind === "failed") {
			logo = { kind: "failed", failure: outcome.failure };

			return;
		}

		await invalidateAll();
		logo = { kind: "idle" };
	}

	async function clearLogo() {
		logo = { kind: "removing" };

		const outcome = await removeLogo(workspace.id);

		if (outcome.kind === "failed") {
			logo = { kind: "failed", failure: outcome.failure };

			return;
		}

		await invalidateAll();
		logo = { kind: "idle" };
	}
</script>

<svelte:head><title>General · {workspace.name} · Norn</title></svelte:head>

<SettingsPage
	title="General"
	description="Identity, defaults, storage and workspace lifecycle."
	Icon={Settings}
	meta={workspace.slug}
	width="compact"
>
	{#snippet toolbar()}
		<SaveBar {bar} {formId} ondiscard={discard} />
	{/snippet}

	{#if pending}
		<Alert.Root variant="destructive">
			<TriangleAlert aria-hidden="true" />
			<Alert.Title>This workspace is scheduled for deletion</Alert.Title>
			<Alert.Description>
				Everything in {workspace.name} is removed permanently on {purgeDate(pending.purgeAfter, workspace.timezone)}.
				Until then an administrator can bring it back, and nobody can change anything in it.
			</Alert.Description>
			{#if administrator}
				<Alert.Action>
					<form method="POST" action="?/restore" use:changeEnhance={trackChange}>
						<input type="hidden" name="workspaceId" value={workspace.id} />
						<Button type="submit" variant="secondary" size="sm" disabled={busy}>
							{changing ? "Restoring" : "Restore this workspace"}
						</Button>
					</form>
				</Alert.Action>
			{/if}
		</Alert.Root>
	{/if}

	{#if !administrator || settings.kind === "forbidden"}
		<Alert.Root variant="muted">
			<div class="col-span-full flex flex-col gap-0.5">
				<Eyebrow>No access · admin only</Eyebrow>
				<Alert.Title>Only administrators change these settings</Alert.Title>
				<Alert.Description>
					You can see how {workspace.name} is set up. Ask an administrator of {workspace.name} to change it.
				</Alert.Description>
			</div>
		</Alert.Root>
	{/if}

	{#if settings.kind === "unavailable"}
		<Alert.Root variant="destructive">
			<CircleX aria-hidden="true" />
			<Alert.Title>Something went wrong</Alert.Title>
			<Alert.Description>Nothing changed. Wait a moment and try again.</Alert.Description>
		</Alert.Root>
	{/if}

	<form
		id={formId}
		method="POST"
		action="?/save"
		use:enhance
		class="flex flex-col gap-6.5"
		aria-label="Workspace settings"
	>
		<input type="hidden" name="workspaceId" value={workspace.id} />
		<input type="hidden" name="previousSlug" value={savedSlug} />
		<input type="hidden" name="timezone" value={$formData.timezone} />
		<input type="hidden" name="weekStartsOn" value={$formData.weekStartsOn} />
		<input type="hidden" name="defaultTeamId" value={$formData.defaultTeamId} />

		<section class="flex flex-col gap-3.5" aria-labelledby="workspace-section">
			<h2><Eyebrow id="workspace-section">Workspace</Eyebrow></h2>

			<div class="flex flex-wrap items-center gap-3">
				<WorkspaceMark name={$formData.name || workspace.name} {logoUrl} class="size-11 text-xl" />
				<div class="flex min-w-0 flex-1 flex-col gap-1.5">
					<div class="flex flex-wrap items-center gap-2">
						<input
							bind:this={logoInput}
							type="file"
							accept={logoAccept}
							class="sr-only"
							tabindex="-1"
							aria-hidden="true"
							onchange={replaceLogo}
						/>
						<Button
							variant="secondary"
							size="sm"
							disabled={locked || logoBusy}
							onclick={() => logoInput?.click()}
						>
							{logo.kind === "uploading" ? "Uploading" : "Upload"}
						</Button>
						<Button
							variant="ghost"
							size="sm"
							disabled={locked || logoBusy || !workspace.logoUrl}
							onclick={clearLogo}
						>
							{logo.kind === "removing" ? "Removing" : "Remove"}
						</Button>
					</div>
					<p class="text-sm text-muted-foreground">PNG, JPEG or WebP, at least 128px square.</p>
					{#if logo.kind === "failed"}
						<p class="text-sm text-destructive" role="alert">{logoFailureMessage(logo.failure)}</p>
					{/if}
				</div>
			</div>

			<Form.Field {form} name="name">
				<Form.Control>
					{#snippet children({ props })}
						<Form.Label>Name</Form.Label>
						<Input
							{...props}
							autocomplete="organization"
							disabled={busy || locked}
							bind:value={$formData.name}
						/>
					{/snippet}
				</Form.Control>
				<Form.Description>Shown in the sidebar and on invitations.</Form.Description>
				<Form.FieldErrors />
			</Form.Field>

			<Form.Field {form} name="slug">
				<Form.Control>
					{#snippet children({ props })}
						<Form.Label>Identifier</Form.Label>
						<div class="flex min-w-0 items-center gap-2">
							<span class="shrink-0 font-mono text-sm text-muted-foreground">{host}/</span>
							<Input
								{...props}
								class="min-w-0 flex-1 font-mono"
								autocapitalize="none"
								autocomplete="off"
								spellcheck="false"
								disabled={busy || locked}
								bind:value={$formData.slug}
							/>
						</div>
					{/snippet}
				</Form.Control>
				<Form.Description>
					Lowercase letters, numbers and dashes. It is the workspace address.
				</Form.Description>
				<Form.FieldErrors />
			</Form.Field>

			{#if movingAddress}
				<Alert.Root variant="warning">
					<TriangleAlert aria-hidden="true" />
					<Alert.Title>Old links keep working for {slugRedirectDays} days</Alert.Title>
					<Alert.Description>
						{host}/{savedSlug} redirects until {redirectEnds(data.now, workspace.timezone)}, then stops.
						{#if exampleKey}Issue ids like {exampleKey}-241 do not change.{:else}Issue ids do not change.{/if}
					</Alert.Description>
				</Alert.Root>
			{/if}
		</section>

		<section class="flex flex-col gap-3.5" aria-labelledby="time-section">
			<h2><Eyebrow id="time-section">Time and defaults</Eyebrow></h2>

			<div class="flex flex-col divide-y divide-line-default rounded-md border border-line-default">
				<SettingRow
					{form}
					name="timezone"
					label="Timezone"
					hint="Cycle boundaries, due dates and digests follow it."
				>
					{#snippet control(props)}
						<Select.Root
							type="single"
							value={$formData.timezone}
							disabled={busy || locked}
							onValueChange={(value) => ($formData.timezone = value)}
						>
							<Select.Trigger {...props} class="w-full">{$formData.timezone}</Select.Trigger>
							<Select.Content>
								{#each zones as zone (zone)}
									<Select.Item value={zone} label={zone}>{zone}</Select.Item>
								{/each}
							</Select.Content>
						</Select.Root>
					{/snippet}
				</SettingRow>

				<SettingRow {form} name="weekStartsOn" label="Week starts on">
					{#snippet control(props)}
						<Select.Root
							type="single"
							value={$formData.weekStartsOn}
							disabled={busy || locked}
							onValueChange={(value) => ($formData.weekStartsOn = value as WeekDay)}
						>
							<Select.Trigger {...props} class="w-full">
								{weekDays.find((day) => day.value === $formData.weekStartsOn)?.label}
							</Select.Trigger>
							<Select.Content>
								{#each weekDays as day (day.value)}
									<Select.Item value={day.value} label={day.label}>{day.label}</Select.Item>
								{/each}
							</Select.Content>
						</Select.Root>
					{/snippet}
				</SettingRow>

				<SettingRow
					{form}
					name="defaultTeamId"
					label="Default team for new members"
					hint={teams.length === 0 ? "This workspace has no teams yet." : undefined}
				>
					{#snippet control(props)}
						<Select.Root
							type="single"
							value={$formData.defaultTeamId}
							disabled={busy || locked || teams.length === 0}
							onValueChange={(value) => ($formData.defaultTeamId = value)}
						>
							<Select.Trigger {...props} class="w-full">
								{teams.find((team) => team.id === $formData.defaultTeamId)?.name ?? "No default team"}
							</Select.Trigger>
							<Select.Content>
								{#each teams as team (team.id)}
									<Select.Item value={team.id} label={team.name}>
										<TeamKey key={team.key} />
										{team.name}
									</Select.Item>
								{/each}
							</Select.Content>
						</Select.Root>
					{/snippet}
				</SettingRow>
			</div>
		</section>

		<section class="flex flex-col gap-3.5" aria-labelledby="agents-section">
			<h2><Eyebrow id="agents-section">Agent instructions</Eyebrow></h2>

			<Form.Field {form} name="agentInstructions">
				<Form.Control>
					{#snippet children({ props })}
						<Form.Label>Instructions for agents in this workspace</Form.Label>
						<Textarea
							{...props}
							rows={10}
							bind:value={$formData.agentInstructions}
							disabled={busy || locked}
						/>
					{/snippet}
				</Form.Control>
				<Form.Description class="text-sm text-muted-foreground">
					Written the way an AGENTS.md file is. Saved for now and nothing else: no agent reads
					them yet. When they are put to work, a project's and an agent's own instructions will
					be added to these rather than replace them.
				</Form.Description>
				<Form.FieldErrors />
			</Form.Field>
		</section>
	</form>

	<section class="flex flex-col gap-3.5" aria-labelledby="storage-section">
		<h2><Eyebrow id="storage-section">Storage</Eyebrow></h2>

		<div class="flex flex-col gap-2 rounded-md border border-line-default px-3 py-2.5">
			<p class="text-sm leading-normal text-muted-foreground text-pretty">
				Files attached to issues in {workspace.name} and files brought in by an import count towards
				this. Norn refuses an upload once there is no room left, so it is worth knowing before you
				reach it.
			</p>

			{#if storage.kind === "unavailable"}
				<p class="text-sm leading-normal text-muted-foreground text-pretty">
					Norn could not read how much this workspace is storing. Wait a moment and try again.
				</p>
			{:else}
				<div class="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-0.5">
					<span class="text-md text-ink-900">{storedLabel(storage)}</span>
					{#if storage.kind === "metered"}
						<span class="text-sm tabular-nums text-muted-foreground">{storage.percent}% used</span>
					{/if}
				</div>

				{#if storage.kind === "metered"}
					<Progress value={storage.percent} tone={storageTone} aria-label="Storage used in this workspace" />
				{/if}

				<p class={headroomClass}>{headroomLabel(storage)}</p>

				<p class="text-sm leading-normal text-muted-foreground text-pretty">
					{measuredLabel(storage, workspace.timezone)}
				</p>
			{/if}
		</div>
	</section>

	{#if !pending && administrator}
		<section class="flex flex-col gap-3.5" aria-labelledby="delete-section">
			<h2><Eyebrow id="delete-section" tone="danger">Delete workspace</Eyebrow></h2>

			<div class="flex flex-col items-start gap-3 rounded-md border border-line-strong p-3.5">
				<div class="flex flex-col gap-1">
					<h3 class="text-md font-medium tracking-snug text-ink-900">
						Delete {workspace.name} and everything in it
					</h3>
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						Deleting locks {workspace.name} straight away and schedules it for permanent removal. An
						administrator can restore it until that date passes. After that it and everything in it
						are gone for good.
					</p>
				</div>
				<Button variant="destructive" size="sm" onclick={() => (confirmingDelete = true)}>
					Delete workspace
				</Button>
			</div>
		</section>
	{/if}
</SettingsPage>

<AlertDialog.Root
	open={confirmingDelete}
	onOpenChange={(open) => {
		if (!open && !changing) {
			confirmingDelete = false;
			confirmation = "";
		}
	}}
>
	<AlertDialog.Content size="sm">
		<form method="POST" action="?/delete" use:changeEnhance={trackChange} class="flex flex-col gap-4">
			<input type="hidden" name="workspaceId" value={workspace.id} />

			<AlertDialog.Header>
				<Eyebrow tone="danger">Locks immediately</Eyebrow>
				<AlertDialog.Title>Delete {workspace.name}</AlertDialog.Title>
				<AlertDialog.Description>
					{teams.length} {teams.length === 1 ? "team" : "teams"} and {data.members.length}
					{data.members.length === 1 ? "member" : "members"} lose access immediately. An administrator
					can restore it until it is removed for good.
				</AlertDialog.Description>
			</AlertDialog.Header>

			{#if dirty}
				<p class="text-sm leading-normal text-warning text-pretty" role="note">
					Unsaved changes on this page are discarded.
				</p>
			{/if}

			<div class="flex flex-col gap-1.5">
				<label for="delete-confirmation" class="text-sm leading-normal text-muted-foreground">
					Type the identifier <span class="font-mono text-ink-900">{savedSlug}</span> to confirm.
				</label>
				<Input
					id="delete-confirmation"
					class="font-mono"
					placeholder={savedSlug}
					autocapitalize="none"
					autocomplete="off"
					spellcheck="false"
					disabled={changing}
					bind:value={confirmation}
				/>
			</div>

			<AlertDialog.Footer>
				<AlertDialog.Cancel disabled={changing}>Cancel</AlertDialog.Cancel>
				<Button type="submit" variant="destructive" disabled={changing || !confirmed}>
					{changing ? "Deleting" : "Delete workspace"}
				</Button>
			</AlertDialog.Footer>
		</form>
	</AlertDialog.Content>
</AlertDialog.Root>

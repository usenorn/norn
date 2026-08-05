<script lang="ts">
	import { goto, invalidateAll } from "$app/navigation";
	import { page } from "$app/state";
	import { defaults, setError, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import CircleCheck from "@lucide/svelte/icons/circle-check";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import Settings from "@lucide/svelte/icons/settings";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Form from "$lib/components/ui/form/index.js";
	import * as Select from "$lib/components/ui/select/index.js";
	import TeamKey from "$lib/components/norn/team-key.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { api } from "$lib/api";
	import { workspacePath } from "$lib/workspace/navigation";
	import { workspaceSettingsSchema } from "$lib/workspace/settings-schema";
	import {
		nameMessage,
		purgeDate,
		settingsFor,
		timezoneMessage,
		timezones,
		type WorkspaceSettings,
	} from "$lib/workspace/settings";
	import { workspaceSettingsPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	const formId = "workspace-settings-form";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV
			? workspaceSettingsPreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined
	);

	let submitted = $state<WorkspaceSettings | null>(null);
	let confirmation = $state("");
	let deleting = $state(false);

	const settings = $derived<WorkspaceSettings>(submitted ?? preview?.settings ?? data.settings);
	const workspace = $derived(settings.workspace);
	const pending = $derived(settings.kind === "pending_deletion" ? settings : null);
	const teams = $derived(preview?.teams ?? data.teams);
	const zones = timezones();

	const form = superForm(defaults(zod4(workspaceSettingsSchema)), {
		id: formId,
		SPA: true,
		validators: zod4Client(workspaceSettingsSchema),
		resetForm: false,
		onUpdate: async ({ form: pendingForm }) => {
			if (!pendingForm.valid) return;

			try {
				const { data: updated, error } = await api.PATCH("/workspaces/{workspaceId}", {
					params: { path: { workspaceId: workspace.id } },
					body: {
						name: pendingForm.data.name,
						timezone: pendingForm.data.timezone,
						defaultTeamId: pendingForm.data.defaultTeamId || undefined,
					},
				});

				if (updated) {
					submitted = { kind: "saved", workspace: updated };
					await invalidateAll();

					return;
				}

				if (error && "code" in error && error.code === "workspace_deleted") {
					submitted = settingsFor({ ...workspace, status: "pending_deletion" });

					return;
				}

				for (const field of error?.errors ?? []) {
					if (field.field === "name") setError(pendingForm, "name", nameMessage(field.code));
					if (field.field === "timezone") {
						setError(pendingForm, "timezone", timezoneMessage(field.code));
					}
					if (field.field === "defaultTeamId") {
						setError(pendingForm, "defaultTeamId", "That team cannot be the default.");
					}
				}
			} catch {
				submitted = { kind: "unavailable", workspace };
			}
		},
	});
	const { form: formData, enhance, submitting } = form;

	$effect(() => {
		const { name, timezone } = workspace;
		const defaultTeamId = workspace.defaultTeamId ?? "";
		formData.update((current) => ({ ...current, name, timezone, defaultTeamId }), {
			taint: false,
		});
	});

	const busy = $derived($submitting || deleting);
	const confirmed = $derived(confirmation.trim() === workspace.slug);

	async function remove() {
		if (!confirmed) return;

		deleting = true;

		try {
			const { data: deleted, error } = await api.DELETE("/workspaces/{workspaceId}", {
				params: { path: { workspaceId: workspace.id } },
			});

			if (deleted) {
				submitted = settingsFor(deleted);
				confirmation = "";
				await invalidateAll();

				return;
			}

			if (error) submitted = { kind: "unavailable", workspace };
		} catch {
			submitted = { kind: "unavailable", workspace };
		} finally {
			deleting = false;
		}
	}

	async function restore() {
		deleting = true;

		try {
			const { data: restored, error } = await api.POST("/workspaces/{workspaceId}/restore", {
				params: { path: { workspaceId: workspace.id } },
			});

			if (restored) {
				submitted = settingsFor(restored);
				await invalidateAll();

				return;
			}

			if (error) submitted = { kind: "unavailable", workspace };
		} catch {
			submitted = { kind: "unavailable", workspace };
		} finally {
			deleting = false;
		}
	}
</script>

<svelte:head><title>Settings · {workspace.name} · Norn</title></svelte:head>

<div class="flex min-h-0 flex-1 flex-col">
	<div class="flex-none border-b border-line-default">
		<div class="flex h-11 items-center gap-2 pr-3 pl-4">
			<Settings class="size-icon-toolbar shrink-0 text-muted-foreground" aria-hidden="true" />
			<h1 class="text-md font-medium tracking-snug whitespace-nowrap text-ink-900">Settings</h1>
		</div>
	</div>

	<div class="flex-1 overflow-auto">
		<div
			class="mx-auto flex w-full max-w-140 flex-col gap-6 px-4 py-6 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))]"
		>
			{#if pending}
				<Alert.Root variant="destructive">
					<TriangleAlert aria-hidden="true" />
					<Alert.Title>This workspace is scheduled for deletion</Alert.Title>
					<Alert.Description>
						Everything in {workspace.name} is removed permanently on {purgeDate(pending.purgeAfter, workspace.timezone)}.
						Until then an administrator can bring it back, and nobody can change anything in it.
					</Alert.Description>
					<Alert.Action>
						<Button variant="secondary" size="sm" disabled={busy} onclick={restore}>
							{deleting ? "Restoring" : "Restore this workspace"}
						</Button>
					</Alert.Action>
				</Alert.Root>
			{/if}

			{#if settings.kind === "saved"}
				<Alert.Root variant="success">
					<CircleCheck aria-hidden="true" />
					<Alert.Title>Settings saved</Alert.Title>
					<Alert.Description>Everyone sees the change immediately.</Alert.Description>
				</Alert.Root>
			{/if}

			{#if settings.kind === "unavailable"}
				<Alert.Root variant="destructive">
					<CircleX aria-hidden="true" />
					<Alert.Title>Something went wrong</Alert.Title>
					<Alert.Description>
						Nothing changed. Wait a moment and try again.
					</Alert.Description>
				</Alert.Root>
			{/if}

			<section class="flex flex-col gap-4">
				<div class="flex flex-col gap-1">
					<h2 class="text-md font-medium tracking-snug text-ink-900">General</h2>
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						How this workspace appears, and the timezone its dates are read in.
					</p>
				</div>

				<form id={formId} method="POST" use:enhance class="flex flex-col gap-4">
					<Form.Field {form} name="name">
						<Form.Control>
							{#snippet children({ props })}
								<Form.Label>Workspace name</Form.Label>
								<Input
									{...props}
									autocomplete="organization"
									disabled={busy || pending !== null}
									bind:value={$formData.name}
								/>
							{/snippet}
						</Form.Control>
						<Form.FieldErrors />
					</Form.Field>

					<div class="flex flex-col gap-1">
						<span class="text-sm font-medium text-ink-900">Address</span>
						<span class="font-mono text-md break-all text-muted-foreground">
							{workspace.slug}
						</span>
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							The address is permanent. It appears in every issue reference people have already
							quoted elsewhere, so changing it would break them.
						</p>
					</div>

					<Form.Field {form} name="timezone">
						<Form.Control>
							{#snippet children({ props })}
								<Form.Label>Timezone</Form.Label>
								<Select.Root
									type="single"
									value={$formData.timezone}
									disabled={busy || pending !== null}
									onValueChange={(value) => ($formData.timezone = value)}
								>
									<Select.Trigger {...props}>{$formData.timezone}</Select.Trigger>
									<Select.Content>
										{#each zones as zone (zone)}
											<Select.Item value={zone} label={zone}>{zone}</Select.Item>
										{/each}
									</Select.Content>
								</Select.Root>
							{/snippet}
						</Form.Control>
						<Form.FieldErrors />
					</Form.Field>
				</form>

				<div>
					<Button type="submit" form={formId} disabled={busy || pending !== null}>
						{$submitting ? "Saving" : "Save changes"}
					</Button>
				</div>
			</section>

			<section class="flex flex-col gap-4">
				<div class="flex flex-col gap-1">
					<h2 class="text-md font-medium tracking-snug text-ink-900">Members</h2>
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						Who is in this workspace, what they can do, and how they sign in.
					</p>
				</div>

				<div>
					<Button variant="secondary" href={workspacePath(workspace.slug, "/settings/members")}>
						Manage members
					</Button>
				</div>
			</section>

			<section class="flex flex-col gap-4">
				<div class="flex flex-col gap-1">
					<h2 class="text-md font-medium tracking-snug text-ink-900">Teams</h2>
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						Teams own issues. New members join the default team automatically.
					</p>
				</div>

				<Form.Field {form} name="defaultTeamId">
					<Form.Control>
						{#snippet children({ props })}
							<Form.Label>Default team for new members</Form.Label>
							<Select.Root
								type="single"
								value={$formData.defaultTeamId}
								disabled={busy || pending !== null || teams.length === 0}
								onValueChange={(value) => ($formData.defaultTeamId = value)}
							>
								<Select.Trigger {...props}>
									{teams.find((team) => team.id === $formData.defaultTeamId)?.name ??
										"No default team"}
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
					</Form.Control>
					{#if teams.length === 0}
						<Form.Description class="text-sm text-muted-foreground">
							This workspace has no teams yet.
						</Form.Description>
					{/if}
					<Form.FieldErrors />
				</Form.Field>

				<div>
					<Button variant="secondary" href={workspacePath(workspace.slug, "/settings/teams")}>
						Manage teams
					</Button>
				</div>
			</section>

			<section class="flex flex-col gap-4">
				<div class="flex flex-col gap-1">
					<h2 class="text-md font-medium tracking-snug text-ink-900">Authentication</h2>
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						Let members sign in through your identity provider instead of a Norn password. Free on
						every instance, including self-hosted.
					</p>
				</div>

				<div>
					<Button
						variant="secondary"
						href={workspacePath(workspace.slug, "/settings/authentication")}
					>
						Set up single sign-on
					</Button>
				</div>
			</section>

			<section class="flex flex-col gap-4 rounded-lg border border-line-subtle p-4">
				<div class="flex flex-col gap-1">
					<h2 class="text-md font-medium tracking-snug text-ink-900">Agents</h2>
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						Agents act under a person's authority and never carry more than that person can do.
						Teams can hold what they change until somebody agrees.
					</p>
				</div>

				<div class="flex flex-wrap gap-2">
					<Button variant="secondary" href={workspacePath(workspace.slug, "/settings/agents")}>
						Manage agents
					</Button>
					<Button variant="ghost" href={workspacePath(workspace.slug, "/agents/approvals")}>
						Waiting for approval
					</Button>
				</div>
			</section>

			<section class="flex flex-col gap-4 rounded-lg border border-line-subtle p-4">
				<div class="flex flex-col gap-1">
					<h2 class="text-md font-medium tracking-snug text-ink-900">Audit log</h2>
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						Every sign-in, membership change, token and configuration change is recorded and cannot
						be altered afterwards. Reading it is granted per person, separately from administering
						{workspace.name}.
					</p>
				</div>

				<div class="flex flex-wrap gap-2">
					<Button variant="secondary" href={workspacePath(workspace.slug, "/settings/audit")}>
						Read the audit log
					</Button>
					<Button variant="ghost" href={workspacePath(workspace.slug, "/settings/members")}>
						Who can read it
					</Button>
				</div>
			</section>

			<section class="flex flex-col gap-4 rounded-lg border border-line-subtle p-4">
				<div class="flex flex-col gap-1">
					<h2 class="text-md font-medium tracking-snug text-ink-900">API tokens</h2>
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						Tokens belong to you rather than to a workspace, and can reach several at once. They
						never carry more than you can do yourself.
					</p>
				</div>

				<div class="flex flex-wrap gap-2">
					<Button variant="secondary" href="/settings/tokens">Your tokens</Button>
					<Button variant="ghost" href={workspacePath(workspace.slug, "/settings/members")}>
						Tokens reaching {workspace.name}
					</Button>
				</div>
			</section>

			{#if !pending}
				<section class="flex flex-col gap-4 rounded-lg border border-destructive/40 p-4">
					<div class="flex flex-col gap-1">
						<h2 class="text-md font-medium tracking-snug text-ink-900">Delete this workspace</h2>
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							Deleting locks {workspace.name} straight away and schedules it for permanent removal.
							An administrator can restore it until that date passes. After that it and everything
							in it are gone for good.
						</p>
					</div>

					<div class="flex flex-col gap-1.5">
						<label
							for="delete-confirmation"
							class="text-sm leading-normal text-muted-foreground text-pretty"
						>
							Type <span class="font-mono text-ink-900">{workspace.slug}</span> to confirm.
						</label>
						<Input
							id="delete-confirmation"
							autocapitalize="none"
							spellcheck="false"
							disabled={busy}
							bind:value={confirmation}
						/>
					</div>

					<div>
						<Button variant="destructive" disabled={busy || !confirmed} onclick={remove}>
							{deleting ? "Deleting" : "Delete workspace"}
						</Button>
					</div>
				</section>
			{/if}
		</div>
	</div>
</div>

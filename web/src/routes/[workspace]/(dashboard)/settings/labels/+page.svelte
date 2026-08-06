<script lang="ts">
	import { goto, invalidate } from "$app/navigation";
	import { keys } from "$lib/api/keys";
	import { page } from "$app/state";
	import { superForm } from "sveltekit-superforms";
	import { zod4Client } from "sveltekit-superforms/adapters";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import Merge from "@lucide/svelte/icons/merge";
	import Plus from "@lucide/svelte/icons/plus";
	import Tags from "@lucide/svelte/icons/tags";
	import X from "@lucide/svelte/icons/x";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Form from "$lib/components/ui/form/index.js";
	import * as Select from "$lib/components/ui/select/index.js";
	import Tag from "$lib/components/norn/tag.svelte";
	import TeamKey from "$lib/components/norn/team-key.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { api } from "$lib/api";
	import {
		colorLabels,
		conflictFailure,
		groupsOf,
		labelColors,
		labelFailureMessage,
		labelsOf,
		mergeTargets,
		sectioned,
		type Label,
		type LabelBoard,
		type LabelColor,
		type LabelFailure,
	} from "$lib/labels/labels";
	import { labelGroupSchema, labelSchema } from "$lib/labels/label-schema";
	import { workspacePath } from "$lib/workspace/navigation";
	import { labelsPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	const labelFormId = "label-form";
	const groupFormId = "label-group-form";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV
			? labelsPreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined
	);

	let directFailure = $state<LabelFailure | null>(null);
	let announcement = $state("");
	let editingId = $state("");
	let working = $state("");
	let usage = $state<number | null>(null);
	let mergeInto = $state("");

	const board = $derived<LabelBoard>(preview?.board ?? data.board);
	const labels = $derived(labelsOf(board));
	const groups = $derived(groupsOf(board));
	const sections = $derived(sectioned(labels, groups));
	const teams = $derived(preview?.teams ?? data.teams);
	const slug = $derived(page.params.workspace ?? "");

	const removalId = $derived(page.url.searchParams.get("remove") ?? "");
	const mergeId = $derived(page.url.searchParams.get("merge") ?? "");
	const removing = $derived(labels.find((label) => label.id === removalId) ?? null);
	const merging = $derived(labels.find((label) => label.id === mergeId) ?? null);
	const targets = $derived(merging ? mergeTargets(merging, labels) : []);
	const editing = $derived(labels.find((label) => label.id === editingId) ?? null);

	const busy = $derived(working !== "");

	function teamKeyOf(teamId: string | undefined): string {
		return teams.find((team) => team.id === teamId)?.key ?? "";
	}

	async function refresh() {
		await invalidate(keys.page(page.route.id));
	}

	function readFailure(error: unknown): LabelFailure {
		if (error && typeof error === "object" && "code" in error) {
			const problem = error as { code: string; issues?: number; status?: number };
			const conflict = conflictFailure(problem.code, problem.issues);

			if (conflict) return conflict;
		}

		if (error && typeof error === "object" && "status" in error) {
			if ((error as { status: number }).status === 403) return { kind: "forbidden" };
		}

		return { kind: "unavailable" };
	}

	// svelte-ignore state_referenced_locally
	const form = superForm(data.form, {
		id: labelFormId,
		validators: zod4Client(labelSchema),
		resetForm: false,
		onSubmit: clearFailure,
		onError: () => (directFailure = { kind: "unavailable" }),
		onUpdated: ({ form: result }) => {
			if (!result.valid || result.message) return;

			announcement = editing
				? `${result.data.name} was saved.`
				: `${result.data.name} was added.`;
			stopEditing();
		},
	});
	const { form: formData, enhance, submitting, message } = form;

	// svelte-ignore state_referenced_locally
	const groupForm = superForm(data.groupForm, {
		id: groupFormId,
		validators: zod4Client(labelGroupSchema),
		resetForm: true,
		onSubmit: clearFailure,
		onError: () => (directFailure = { kind: "unavailable" }),
		onUpdated: ({ form: result }) => {
			if (!result.valid || result.message) return;

			announcement = `${result.data.name} was added. Labels in it are mutually exclusive.`;
		},
	});
	const {
		form: groupData,
		enhance: groupEnhance,
		submitting: groupSubmitting,
		message: groupMessage,
	} = groupForm;

	const failure = $derived<LabelFailure | null>(
		directFailure ?? $message ?? $groupMessage ?? null
	);

	function clearFailure() {
		directFailure = null;
		message.set(undefined);
		groupMessage.set(undefined);
	}

	function startEditing(label: Label) {
		editingId = label.id;
		clearFailure();
		formData.set(
			{
				name: label.name,
				color: label.color,
				groupId: label.groupId ?? "",
				teamId: label.teamId ?? "",
			},
			{ taint: false }
		);
	}

	function stopEditing() {
		editingId = "";
		formData.set({ name: "", color: "cyan", groupId: "", teamId: "" }, { taint: false });
	}

	function panelHref(key: "remove" | "merge", label: Label): string {
		const next = new URL(page.url);
		next.searchParams.delete(key === "remove" ? "merge" : "remove");
		next.searchParams.set(key, label.id);

		return `${next.pathname}${next.search}`;
	}

	async function closePanels() {
		const next = new URL(page.url);
		next.searchParams.delete("remove");
		next.searchParams.delete("merge");
		usage = null;
		mergeInto = "";

		await goto(next, { replaceState: true, noScroll: true });
	}

	$effect(() => {
		const id = removalId;

		if (!id) {
			usage = null;

			return;
		}

		usage = null;

		api
			.GET("/workspaces/{workspaceId}/labels/{labelId}/usage", {
				params: { path: { workspaceId: data.workspace.id, labelId: id } },
			})
			.then(({ data: read }) => {
				if (read) usage = read.issues;
			})
			.catch(() => {
				directFailure = { kind: "unavailable" };
			});
	});

	async function confirmRemoval() {
		if (!removing || usage === null) return;

		working = removing.id;
		clearFailure();

		try {
			const { error } = await api.DELETE("/workspaces/{workspaceId}/labels/{labelId}", {
				params: {
					path: { workspaceId: data.workspace.id, labelId: removing.id },
					query: { acknowledgedIssues: usage },
				},
			});

			if (error) {
				const conflict = readFailure(error);
				directFailure = conflict;

				if (conflict.kind === "usage_changed") usage = conflict.issues;

				return;
			}

			announcement = `${removing.name} was removed from ${usage} ${usage === 1 ? "issue" : "issues"}.`;
			await closePanels();
			await refresh();
		} catch {
			directFailure = { kind: "unavailable" };
		} finally {
			working = "";
		}
	}

	async function confirmMerge() {
		if (!merging || !mergeInto) return;

		working = merging.id;
		clearFailure();

		try {
			const { data: kept, error } = await api.POST(
				"/workspaces/{workspaceId}/labels/{labelId}/merge",
				{
					params: { path: { workspaceId: data.workspace.id, labelId: merging.id } },
					body: { intoLabelId: mergeInto },
				}
			);

			if (error) {
				directFailure = readFailure(error);

				return;
			}

			announcement = `${merging.name} was merged into ${kept?.name ?? "the other label"}.`;
			await closePanels();
			await refresh();
		} catch {
			directFailure = { kind: "unavailable" };
		} finally {
			working = "";
		}
	}

	async function removeGroup(id: string, name: string) {
		working = id;
		clearFailure();

		try {
			const { error } = await api.DELETE("/workspaces/{workspaceId}/label-groups/{groupId}", {
				params: { path: { workspaceId: data.workspace.id, groupId: id } },
			});

			if (error) {
				directFailure = readFailure(error);

				return;
			}

			announcement = `${name} was removed. Its labels are still here, no longer exclusive.`;
			await refresh();
		} catch {
			directFailure = { kind: "unavailable" };
		} finally {
			working = "";
		}
	}
</script>

<svelte:head><title>Labels · {data.workspace.name} · Norn</title></svelte:head>

<div class="flex min-h-0 flex-1 flex-col">
	<div class="flex-none border-b border-line-default">
		<div class="flex h-11 items-center gap-2 pr-3 pl-4">
			<Tags class="size-icon-toolbar shrink-0 text-muted-foreground" aria-hidden="true" />
			<h1 class="text-md font-medium tracking-snug whitespace-nowrap text-ink-900">Labels</h1>
		</div>
	</div>

	<div class="flex-1 overflow-auto">
		<div
			class="mx-auto flex w-full max-w-140 flex-col gap-6 px-4 py-6 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))]"
		>
			<p class="sr-only" role="status" aria-live="polite">{announcement}</p>

			{#if failure}
				<Alert.Root variant="destructive">
					<CircleX aria-hidden="true" />
					<Alert.Title>That did not work</Alert.Title>
					<Alert.Description>{labelFailureMessage(failure)}</Alert.Description>
				</Alert.Root>
			{/if}

			<section class="flex flex-col gap-4">
				<div class="flex flex-col gap-1">
					<h2 class="text-md font-medium tracking-snug text-ink-900">Labels</h2>
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						Labels cross every team and project. A label can be narrowed to one team, and labels in
						a group are mutually exclusive — an issue carries at most one of them.
					</p>
				</div>

				{#if board.kind === "loading"}
					<div class="h-40 animate-pulse rounded-lg bg-paper-2" aria-busy="true"></div>
				{:else if board.kind === "unavailable"}
					<p class="text-sm leading-normal text-muted-foreground">
						We could not load this workspace's labels.
					</p>
				{:else if labels.length === 0 && groups.length === 0}
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						No labels yet. Add the first one below — it becomes available on every issue in
						{data.workspace.name}.
					</p>
				{:else}
					{#each sections as section (section.group?.id ?? "ungrouped")}
						<div class="flex flex-col gap-2">
							<div class="flex items-center gap-2">
								<span
									class="font-mono text-2xs font-medium tracking-eyebrow text-ink-600 uppercase"
								>
									{section.group ? section.group.name : "Ungrouped"}
								</span>
								{#if section.group}
									<span class="text-sm text-muted-foreground">one per issue</span>
									<span class="h-px flex-1 bg-line-default" aria-hidden="true"></span>
									<Button
										variant="ghost"
										size="sm"
										disabled={busy}
										onclick={() => removeGroup(section.group!.id, section.group!.name)}
									>
										Ungroup these
									</Button>
								{:else}
									<span class="h-px flex-1 bg-line-default" aria-hidden="true"></span>
								{/if}
							</div>

							{#if section.labels.length === 0}
								<p class="text-sm leading-normal text-muted-foreground">
									Nothing in this group yet.
								</p>
							{:else}
								<ul class="flex flex-col rounded-lg border border-line-default">
									{#each section.labels as label (label.id)}
										<li class="flex flex-col border-b border-line-subtle last:border-b-0">
											<div class="flex flex-wrap items-center gap-2 px-3 py-2">
												<Tag name={label.name} color={label.color} />
												<span class="min-w-0 flex-[1_1_80px] truncate text-md text-ink-900">
													{label.name}
												</span>

												{#if label.teamId}
													<TeamKey key={teamKeyOf(label.teamId)} />
												{:else}
													<span class="shrink-0 text-sm text-muted-foreground">Workspace</span>
												{/if}

												<span class="flex shrink-0 items-center gap-0.5">
													<Button
														variant="ghost"
														size="sm"
														disabled={busy}
														onclick={() =>
															editingId === label.id ? stopEditing() : startEditing(label)}
													>
														{editingId === label.id ? "Cancel" : "Edit"}
													</Button>
													<Button
														variant="ghost"
														size="icon-sm"
														disabled={busy || mergeTargets(label, labels).length === 0}
														aria-label={mergeTargets(label, labels).length === 0
															? `No label can absorb ${label.name} — a target must be in the same group and cover its scope`
															: `Merge ${label.name} into another label`}
														title={mergeTargets(label, labels).length === 0
															? "A merge target must be in the same group and cover this label's scope."
															: undefined}
														href={panelHref("merge", label)}
													>
														<Merge aria-hidden="true" />
													</Button>
													<Button
														variant="ghost"
														size="icon-sm"
														disabled={busy}
														aria-label="Remove {label.name}"
														href={panelHref("remove", label)}
													>
														<X aria-hidden="true" />
													</Button>
												</span>
											</div>

											{#if removalId === label.id}
												<div
													class="flex flex-col gap-3 border-t border-line-subtle bg-paper-1 px-3 py-3"
												>
													<div class="flex flex-col gap-1">
														<span class="text-sm font-medium text-ink-900">
															Remove {label.name}
														</span>
														{#if usage === null}
															<p class="text-sm text-muted-foreground">
																Counting the issues it is on…
															</p>
														{:else}
															<p
																class="text-sm leading-normal text-muted-foreground text-pretty"
															>
																{label.name} is on
																<strong class="text-ink-900">
																	{usage}
																	{usage === 1 ? "issue" : "issues"}
																</strong>. Removing it takes it off all of them. Nothing else
																about those issues changes.
															</p>
														{/if}
													</div>

													<div class="flex gap-2">
														<Button
															variant="destructive"
															size="sm"
															disabled={busy || usage === null}
															onclick={confirmRemoval}
														>
															{working === label.id
																? "Removing"
																: usage === null
																	? "Checking"
																	: `Remove from ${usage} ${usage === 1 ? "issue" : "issues"}`}
														</Button>
														<Button
															variant="secondary"
															size="sm"
															disabled={busy}
															onclick={closePanels}
														>
															Keep it
														</Button>
													</div>
												</div>
											{/if}

											{#if mergeId === label.id}
												<div
													class="flex flex-col gap-3 border-t border-line-subtle bg-paper-1 px-3 py-3"
												>
													<div class="flex flex-col gap-1">
														<span class="text-sm font-medium text-ink-900">
															Merge {label.name} into another label
														</span>
														<p class="text-sm leading-normal text-muted-foreground text-pretty">
															Every issue carrying {label.name} moves to the label you choose, and
															{label.name} is removed. Issues already carrying both keep one.
														</p>
													</div>

													{#if targets.length === 0}
														<p class="text-sm leading-normal text-muted-foreground text-pretty">
															Nothing can absorb this label. A target has to be in the same group
															and cover this label's scope, so a workspace label cannot merge into
															a team one.
														</p>
														<div>
															<Button variant="secondary" size="sm" onclick={closePanels}>
																Close
															</Button>
														</div>
													{:else}
														<Select.Root
															type="single"
															value={mergeInto}
															disabled={busy}
															onValueChange={(value) => (mergeInto = value)}
														>
															<Select.Trigger aria-label="Merge into">
																{targets.find((candidate) => candidate.id === mergeInto)?.name ??
																	"Choose a label"}
															</Select.Trigger>
															<Select.Content>
																{#each targets as candidate (candidate.id)}
																	<Select.Item value={candidate.id} label={candidate.name}>
																		{candidate.name}
																	</Select.Item>
																{/each}
															</Select.Content>
														</Select.Root>

														<div class="flex gap-2">
															<Button
																size="sm"
																disabled={busy || !mergeInto}
																onclick={confirmMerge}
															>
																{working === label.id ? "Merging" : "Merge and move issues"}
															</Button>
															<Button
																variant="secondary"
																size="sm"
																disabled={busy}
																onclick={closePanels}
															>
																Cancel
															</Button>
														</div>
													{/if}
												</div>
											{/if}
										</li>
									{/each}
								</ul>
							{/if}
						</div>
					{/each}
				{/if}

				<form
					id={labelFormId}
					method="POST"
					action="?/label"
					use:enhance
					class="flex flex-col gap-4"
				>
					<input type="hidden" name="workspaceId" value={data.workspace.id} />
					<input type="hidden" name="labelId" value={editingId} />

					<h3 class="text-sm font-medium text-ink-900">
						{editing ? `Edit ${editing.name}` : "Add a label"}
					</h3>

					<Form.Field {form} name="name">
						<Form.Control>
							{#snippet children({ props })}
								<Form.Label>Name</Form.Label>
								<Input {...props} disabled={busy} placeholder="Bug" bind:value={$formData.name} />
							{/snippet}
						</Form.Control>
						<Form.FieldErrors />
					</Form.Field>

					<Form.Field {form} name="color">
						<Form.Control>
							{#snippet children({ props })}
								<Form.Label>Colour</Form.Label>
								<div {...props} class="flex flex-wrap gap-2">
									{#each labelColors as color (color)}
										<button
											type="button"
											disabled={busy}
											aria-pressed={$formData.color === color}
											aria-label={colorLabels[color]}
											onclick={() => ($formData.color = color as LabelColor)}
											class="inline-flex items-center gap-1.5 rounded-sm border px-2 py-1 text-sm transition-colors duration-70 ease-out disabled:opacity-50 aria-pressed:border-primary aria-pressed:bg-accent"
											class:border-line-default={$formData.color !== color}
										>
											<span
												class="size-2.5 rounded-xs"
												style="background: var(--label-{color})"
												aria-hidden="true"
											></span>
											{colorLabels[color]}
										</button>
									{/each}
								</div>
								<input type="hidden" name={props.name} value={$formData.color} />
							{/snippet}
						</Form.Control>
						<Form.Description class="text-sm text-muted-foreground">
							These are the only colours labels may use — status and priority own the reds, ambers
							and greens, so a label can never be mistaken for one.
						</Form.Description>
						<Form.FieldErrors />
					</Form.Field>

					<Form.Field {form} name="groupId">
						<Form.Control>
							{#snippet children({ props })}
								<Form.Label>Group</Form.Label>
								<Select.Root
									type="single"
									name={props.name}
									value={$formData.groupId}
									disabled={busy}
									onValueChange={(value) => ($formData.groupId = value)}
								>
									<Select.Trigger {...props}>
										{groups.find((group) => group.id === $formData.groupId)?.name ?? "No group"}
									</Select.Trigger>
									<Select.Content>
										<Select.Item value="" label="No group">No group</Select.Item>
										{#each groups as group (group.id)}
											<Select.Item value={group.id} label={group.name}>{group.name}</Select.Item>
										{/each}
									</Select.Content>
								</Select.Root>
							{/snippet}
						</Form.Control>
						<Form.Description class="text-sm text-muted-foreground">
							An issue carries at most one label from a group.
						</Form.Description>
						<Form.FieldErrors />
					</Form.Field>

					{#if !editing}
						<Form.Field {form} name="teamId">
							<Form.Control>
								{#snippet children({ props })}
									<Form.Label>Scope</Form.Label>
									<Select.Root
										type="single"
										name={props.name}
										value={$formData.teamId}
										disabled={busy}
										onValueChange={(value) => ($formData.teamId = value)}
									>
										<Select.Trigger {...props}>
											{teams.find((team) => team.id === $formData.teamId)?.name ??
												"Every team in this workspace"}
										</Select.Trigger>
										<Select.Content>
											<Select.Item value="" label="Every team in this workspace">
												Every team in this workspace
											</Select.Item>
											{#each teams as team (team.id)}
												<Select.Item value={team.id} label={team.name}>{team.name}</Select.Item>
											{/each}
										</Select.Content>
									</Select.Root>
								{/snippet}
							</Form.Control>
							<Form.Description class="text-sm text-muted-foreground">
								Scope is permanent. To widen or narrow a label later, merge it into one with the
								scope you want.
							</Form.Description>
							<Form.FieldErrors />
						</Form.Field>
					{/if}
				</form>

				<div class="flex gap-2">
					<Button type="submit" form={labelFormId} disabled={busy || $submitting}>
						{#if !editing}
							<Plus aria-hidden="true" />
						{/if}
						{$submitting ? "Saving" : editing ? "Save label" : "Add label"}
					</Button>
					{#if editing}
						<Button variant="secondary" disabled={busy} onclick={stopEditing}>Cancel</Button>
					{/if}
				</div>
			</section>

			<section class="flex flex-col gap-4">
				<div class="flex flex-col gap-1">
					<h2 class="text-md font-medium tracking-snug text-ink-900">Groups</h2>
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						A group makes its labels mutually exclusive, for things an issue can only be one of —
						a severity, a size, a stage.
					</p>
				</div>

				<form
					id={groupFormId}
					method="POST"
					action="?/group"
					use:groupEnhance
					class="flex flex-col gap-4"
				>
					<input type="hidden" name="workspaceId" value={data.workspace.id} />

					<Form.Field form={groupForm} name="name">
						<Form.Control>
							{#snippet children({ props })}
								<Form.Label>Group name</Form.Label>
								<Input
									{...props}
									disabled={busy}
									placeholder="Severity"
									bind:value={$groupData.name}
								/>
							{/snippet}
						</Form.Control>
						<Form.FieldErrors />
					</Form.Field>
				</form>

				<div>
					<Button
						type="submit"
						form={groupFormId}
						variant="secondary"
						disabled={busy || $groupSubmitting}
					>
						<Plus aria-hidden="true" />
						{$groupSubmitting ? "Adding" : "Add group"}
					</Button>
				</div>
			</section>

			<p class="text-sm leading-normal text-muted-foreground text-pretty">
				Labels appear on every issue in
				<a
					href={workspacePath(slug, "/issues")}
					class="text-link underline-offset-2 hover:text-link-hover hover:underline"
				>
					the issue board
				</a>.
			</p>
		</div>
	</div>
</div>

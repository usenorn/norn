<script lang="ts">
	import { invalidate } from "$app/navigation";
	import { keys } from "$lib/api/keys";
	import { page } from "$app/state";
	import { superForm } from "sveltekit-superforms";
	import { zod4Client } from "sveltekit-superforms/adapters";
	import Check from "@lucide/svelte/icons/check";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import List from "@lucide/svelte/icons/list";
	import Merge from "@lucide/svelte/icons/merge";
	import MoreHorizontal from "@lucide/svelte/icons/more-horizontal";
	import Pencil from "@lucide/svelte/icons/pencil";
	import Plus from "@lucide/svelte/icons/plus";
	import Search from "@lucide/svelte/icons/search";
	import Tags from "@lucide/svelte/icons/tags";
	import Trash2 from "@lucide/svelte/icons/trash-2";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import Ungroup from "@lucide/svelte/icons/ungroup";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as DropdownMenu from "$lib/components/ui/dropdown-menu/index.js";
	import * as Empty from "$lib/components/ui/empty/index.js";
	import * as InputGroup from "$lib/components/ui/input-group/index.js";
	import * as Popover from "$lib/components/ui/popover/index.js";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import Tag from "$lib/components/norn/tag.svelte";
	import TeamKey from "$lib/components/norn/team-key.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { Skeleton } from "$lib/components/ui/skeleton/index.js";
	import SettingsPage from "$lib/settings/settings-page.svelte";
	import { api } from "$lib/api";
	import DeleteLabelDialog from "$lib/labels/delete-label-dialog.svelte";
	import LabelEditor from "$lib/labels/label-editor.svelte";
	import MergeLabelDialog from "$lib/labels/merge-label-dialog.svelte";
	import {
		groupsOf,
		issueCount,
		labelFailureMessage,
		labelsOf,
		matches,
		mergeTargets,
		refusedLabelFailure,
		sectioned,
		uncountedLabel,
		usageLabel,
		usageOf,
		type Label,
		type LabelBoard,
		type LabelFailure,
		type LabelGroup,
		type LabelUsage,
		type UsageRead,
	} from "$lib/labels/labels";
	import { labelSchema } from "$lib/labels/label-schema";
	import { workspacePath } from "$lib/workspace/navigation";
	import { labelsPreviewStates } from "./preview";
	import type { PageProps } from "./$types";
	import { showToast } from "$lib/toast/toasts";

	const labelFormId = "label-form";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV
			? labelsPreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined
	);

	let directFailure = $state<LabelFailure | null>(null);
	let editingId = $state("");
	let editorOpen = $state(false);
	let working = $state("");
	let query = $state("");
	let createdGroups = $state<LabelGroup[]>([]);

	let mergeSource = $state<Label | null>(null);
	let mergeOpen = $state(false);
	let removalTarget = $state<Label | null>(null);
	let removalOpen = $state(false);
	let removalUsage = $state<UsageRead>({ kind: "counting" });

	let renamingGroupId = $state("");
	let renamedGroup = $state("");

	const board = $derived<LabelBoard>(preview?.board ?? data.board);
	const labels = $derived(labelsOf(board));
	const groups = $derived.by<LabelGroup[]>(() => {
		const known = groupsOf(board);
		const held = new Set(known.map((group) => group.id));

		return [...known, ...createdGroups.filter((group) => !held.has(group.id))];
	});
	const teams = $derived(preview?.teams ?? data.teams);
	const usage = $derived<LabelUsage>(preview?.usage ?? data.usage);
	const uncounted = $derived(usage.kind === "uncounted");
	const slug = $derived(page.params.workspace ?? "");

	const found = $derived(labels.filter((label) => matches(label, query)));
	const sections = $derived(sectioned(found, groups));
	const editing = $derived(labels.find((label) => label.id === editingId) ?? null);
	const busy = $derived(working !== "");

	let opened = $state("");

	$effect(() => {
		const wanted = preview?.opens;

		if (!wanted || opened === wanted || labels.length === 0) return;

		opened = wanted;

		if (wanted === "editor") openEdit(labels[0]);
		if (wanted === "merge") openMerge(labels[0]);
		if (wanted === "delete") openRemoval(labels[0]);
	});

	const countLine = $derived(
		`${found.length} ${found.length === 1 ? "label" : "labels"} · ${
			groups.length === 0 ? "no groups" : `${groups.length} ${groups.length === 1 ? "group" : "groups"}`
		}`
	);

	function teamKeyOf(teamId: string | undefined): string {
		return teams.find((team) => team.id === teamId)?.key ?? "";
	}

	async function refresh() {
		createdGroups = [];

		await Promise.all([
			invalidate(keys.page(page.route.id)),
			invalidate(keys.labels(data.workspace.id)),
		]);
	}

	function readFailure(error: unknown, status: number): LabelFailure {
		return refusedLabelFailure(error, status);
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

			showToast(editing ? `${result.data.name} was saved.` : `${result.data.name} was added.`);
			closeEditor();
			createdGroups = [];
		},
	});
	const { form: formData, enhance, submitting, message } = form;

	const failure = $derived<LabelFailure | null>(directFailure ?? $message ?? null);

	function clearFailure() {
		directFailure = null;
		message.set(undefined);
	}

	function openCreate() {
		editingId = "";
		clearFailure();
		formData.set(
			{ name: "", description: "", color: "cyan", groupId: "", teamId: "" },
			{ taint: false }
		);
		editorOpen = true;
	}

	function openEdit(label: Label) {
		editingId = label.id;
		clearFailure();
		formData.set(
			{
				name: label.name,
				description: label.description,
				color: label.color,
				groupId: label.groupId ?? "",
				teamId: label.teamId ?? "",
			},
			{ taint: false }
		);
		editorOpen = true;
	}

	function closeEditor() {
		editorOpen = false;
		editingId = "";
	}

	function openMerge(label: Label) {
		mergeSource = label;
		mergeOpen = true;
		clearFailure();
	}

	async function countUsage(label: Label) {
		removalUsage = { kind: "counting" };

		if (preview) {
			removalUsage = { kind: "counted", issues: usageOf(usage, label) ?? 0 };

			return;
		}

		const settle = (read: UsageRead) => {
			if (removalTarget?.id === label.id) removalUsage = read;
		};

		try {
			const { data: read, error, response } = await api.GET(
				"/workspaces/{workspaceId}/labels/{labelId}/usage",
				{ params: { path: { workspaceId: data.workspace.id, labelId: label.id } } }
			);

			if (error || !read) {
				settle({ kind: "refused", failure: readFailure(error, response?.status ?? 0) });

				return;
			}

			settle({ kind: "counted", issues: read.issues });
		} catch {
			settle({ kind: "refused", failure: { kind: "unavailable" } });
		}
	}

	function openRemoval(label: Label) {
		removalTarget = label;
		removalOpen = true;
		clearFailure();
		void countUsage(label);
	}

	function retryUsage() {
		if (removalTarget) void countUsage(removalTarget);
	}

	function mergeInstead() {
		const label = removalTarget;
		removalOpen = false;

		if (label) openMerge(label);
	}

	async function confirmRemoval() {
		const label = removalTarget;
		const counted = removalUsage;

		if (!label || counted.kind !== "counted") return;

		working = label.id;
		clearFailure();

		try {
			const { error, response } = await api.DELETE("/workspaces/{workspaceId}/labels/{labelId}", {
				params: {
					path: { workspaceId: data.workspace.id, labelId: label.id },
					query: { acknowledgedIssues: counted.issues },
				},
			});

			if (error) {
				const conflict = readFailure(error, response?.status ?? 0);
				directFailure = conflict;

				if (conflict.kind === "usage_changed") {
					removalUsage = { kind: "counted", issues: conflict.issues };
				}

				return;
			}

			showToast(`${label.name} was removed from ${issueCount(counted.issues)}.`);
			removalOpen = false;
			removalTarget = null;
			await refresh();
		} catch {
			directFailure = { kind: "unavailable" };
		} finally {
			working = "";
		}
	}

	async function confirmMerge(targetId: string) {
		const source = mergeSource;

		if (!source || !targetId) return;

		working = source.id;
		clearFailure();

		try {
			const { data: kept, error, response } = await api.POST(
				"/workspaces/{workspaceId}/labels/{labelId}/merge",
				{
					params: { path: { workspaceId: data.workspace.id, labelId: source.id } },
					body: { intoLabelId: targetId },
				}
			);

			if (error) {
				directFailure = readFailure(error, response?.status ?? 0);

				return;
			}

			showToast(`${source.name} was merged into ${kept?.name ?? "the other label"}.`);
			mergeOpen = false;
			mergeSource = null;
			await refresh();
		} catch {
			directFailure = { kind: "unavailable" };
		} finally {
			working = "";
		}
	}

	function startRenamingGroup(group: LabelGroup) {
		renamingGroupId = group.id;
		renamedGroup = group.name;
		clearFailure();
	}

	function stopRenamingGroup() {
		renamingGroupId = "";
		renamedGroup = "";
	}

	async function confirmGroupRename(group: LabelGroup) {
		const name = renamedGroup.trim();

		if (!name || name === group.name) {
			stopRenamingGroup();

			return;
		}

		working = group.id;
		clearFailure();

		try {
			const { error, response } = await api.PATCH(
				"/workspaces/{workspaceId}/label-groups/{groupId}",
				{
					params: { path: { workspaceId: data.workspace.id, groupId: group.id } },
					body: { name },
				}
			);

			if (error) {
				directFailure = readFailure(error, response?.status ?? 0);

				return;
			}

			showToast(`${group.name} is now ${name}.`);
			stopRenamingGroup();
			await refresh();
		} catch {
			directFailure = { kind: "unavailable" };
		} finally {
			working = "";
		}
	}

	async function removeGroup(group: LabelGroup) {
		working = group.id;
		clearFailure();

		try {
			const { error, response } = await api.DELETE(
				"/workspaces/{workspaceId}/label-groups/{groupId}",
				{
					params: { path: { workspaceId: data.workspace.id, groupId: group.id } },
				}
			);

			if (error) {
				directFailure = readFailure(error, response?.status ?? 0);

				return;
			}

			showToast(`${group.name} was removed. Its labels are still here, no longer exclusive.`);
			await refresh();
		} catch {
			directFailure = { kind: "unavailable" };
		} finally {
			working = "";
		}
	}
</script>

<svelte:head><title>Labels · {data.workspace.name} · Norn</title></svelte:head>

<SettingsPage
	title="Labels"
	description="Labels cut across teams. They are cool colours only — red, amber and green belong to status, so a label can never be mistaken for one."
	Icon={Tags}
	meta={board.kind === "loading" ? "loading" : countLine}
	width="standard"
>
	{#snippet actions()}
		<Popover.Root
			open={editorOpen}
			onOpenChange={(next) => {
				editorOpen = next;

				if (!next) editingId = "";
			}}
		>
			<Popover.Trigger disabled={busy || board.kind !== "ready"}>
				{#snippet child({ props })}
					<Button {...props} onclick={openCreate}>
						<Plus aria-hidden="true" />
						New label
					</Button>
				{/snippet}
			</Popover.Trigger>
			<Popover.Content align="end" class="w-[min(20rem,calc(100vw-2rem))] p-3">
				<Popover.Header class="pb-2">
					<Popover.Title>{editing ? "Edit label" : "New label"}</Popover.Title>
				</Popover.Header>

				<form id={labelFormId} method="POST" action="?/label" use:enhance>
					<input type="hidden" name="workspaceId" value={data.workspace.id} />
					<input type="hidden" name="labelId" value={editingId} />

					<LabelEditor
						{form}
						formId={labelFormId}
						workspaceId={data.workspace.id}
						{groups}
						{teams}
						editing={editing !== null}
						submitting={$submitting}
						ongroupcreated={(group) => (createdGroups = [...createdGroups, group])}
						oncancel={closeEditor}
					/>
				</form>
			</Popover.Content>
		</Popover.Root>
	{/snippet}

	{#if failure}
		<Alert.Root variant="destructive">
			<CircleX aria-hidden="true" />
			<Alert.Title>That did not work</Alert.Title>
			<Alert.Description>{labelFailureMessage(failure)}</Alert.Description>
		</Alert.Root>
	{/if}

	{#if board.kind === "loading"}
		<div class="flex flex-col gap-2" aria-busy="true" aria-label="Loading labels">
			{#each [0, 1, 2, 3] as row (row)}
				<Skeleton class="h-10 w-full" />
			{/each}
		</div>
	{:else if board.kind === "unavailable"}
		<Alert.Root variant="destructive">
			<CircleX aria-hidden="true" />
			<Alert.Title>Could not load the labels</Alert.Title>
			<Alert.Description>Check your connection and reload.</Alert.Description>
		</Alert.Root>
	{:else if labels.length === 0}
		<Empty.Root>
			<Empty.Media variant="icon"><Tags aria-hidden="true" /></Empty.Media>
			<Empty.Header>
				<Empty.Title>No labels yet</Empty.Title>
				<Empty.Description>
					Labels are for the crosscutting things status cannot say — bug, needs spec, tech debt.
					Four or five is usually the whole set.
				</Empty.Description>
			</Empty.Header>
			<Empty.Content>
				<Button onclick={openCreate}>
					<Plus aria-hidden="true" />
					New label
				</Button>
			</Empty.Content>
		</Empty.Root>
	{:else}
		<div class="flex flex-col gap-4">
			<div class="flex flex-wrap items-center gap-3">
				<InputGroup.Root class="w-full sm:w-55">
					<InputGroup.Addon>
						<Search aria-hidden="true" />
					</InputGroup.Addon>
					<InputGroup.Input
						placeholder="Search labels"
						aria-label="Search labels"
						bind:value={query}
					/>
				</InputGroup.Root>
				<span class="flex-1"></span>
				<span class="font-mono text-2xs text-muted-foreground">{countLine}</span>
			</div>

			{#if uncounted}
				<Alert.Root>
					<TriangleAlert aria-hidden="true" />
					<Alert.Title>Issue counts are unavailable</Alert.Title>
					<Alert.Description>
						The count of issues carrying each label could not be read, so every label shows {uncountedLabel}
						instead of a number. Reload to try again. Everything else on this page still works.
					</Alert.Description>
				</Alert.Root>
			{/if}

			{#if found.length === 0}
				<Empty.Root>
					<Empty.Media variant="icon"><Search aria-hidden="true" /></Empty.Media>
					<Empty.Header>
						<Empty.Title>Nothing matches “{query}”</Empty.Title>
						<Empty.Description>
							Search runs over label names and their descriptions.
						</Empty.Description>
					</Empty.Header>
				</Empty.Root>
			{:else}
				{#each sections as section (section.group?.id ?? "ungrouped")}
					<div class="flex flex-col gap-2">
						<div class="flex items-center gap-2">
							{#if renamingGroupId === section.group?.id}
								<Input
									bind:value={renamedGroup}
									disabled={busy}
									aria-label="Group name"
									class="h-7 w-44"
								/>
								<Button
									size="icon-sm"
									disabled={busy}
									aria-label="Save group name"
									onclick={() => confirmGroupRename(section.group!)}
								>
									<Check aria-hidden="true" />
								</Button>
								<Button variant="ghost" size="sm" disabled={busy} onclick={stopRenamingGroup}>
									Cancel
								</Button>
							{:else}
								<Eyebrow>{section.group ? section.group.name : "Ungrouped"}</Eyebrow>
								{#if section.group}
									<span
										class="rounded-full border border-line-default px-1.5 font-mono text-2xs tracking-eyebrow text-muted-foreground uppercase"
									>
										one at a time
									</span>
								{/if}
								<span class="h-px flex-1 bg-line-subtle" aria-hidden="true"></span>
								<span class="font-mono text-2xs text-muted-foreground">
									{section.labels.length}
								</span>
								{#if section.group}
									<DropdownMenu.Root>
										<DropdownMenu.Trigger disabled={busy}>
											{#snippet child({ props })}
												<Button
													{...props}
													variant="ghost"
													size="icon-sm"
													aria-label={`Actions for ${section.group!.name}`}
												>
													<MoreHorizontal aria-hidden="true" />
												</Button>
											{/snippet}
										</DropdownMenu.Trigger>
										<DropdownMenu.Content align="end">
											<DropdownMenu.Item onSelect={() => startRenamingGroup(section.group!)}>
												<Pencil aria-hidden="true" />
												Rename group
											</DropdownMenu.Item>
											<DropdownMenu.Item
												variant="destructive"
												onSelect={() => removeGroup(section.group!)}
											>
												<Ungroup aria-hidden="true" />
												Ungroup these labels
											</DropdownMenu.Item>
										</DropdownMenu.Content>
									</DropdownMenu.Root>
								{/if}
							{/if}
						</div>

						{#if section.labels.length === 0}
							<p class="text-sm leading-normal text-muted-foreground">Nothing in this group yet.</p>
						{:else}
							<ul class="overflow-hidden rounded-lg border border-line-default bg-paper-0">
								{#each section.labels as label (label.id)}
									<li
										class="flex min-h-10 flex-wrap items-center gap-x-3 gap-y-1 border-b border-line-subtle px-3 py-2 last:border-b-0"
									>
										<span class="flex w-30 shrink-0 items-center">
											<Tag name={label.name} color={label.color} />
										</span>

										<span class="min-w-0 flex-1 truncate text-sm text-muted-foreground">
											{label.description}
										</span>

										{#if label.teamId}
											<TeamKey key={teamKeyOf(label.teamId)} />
										{/if}

										<span class="shrink-0 font-mono text-2xs text-muted-foreground">
											{usageLabel(usage, label)}
										</span>

										<DropdownMenu.Root>
											<DropdownMenu.Trigger disabled={busy}>
												{#snippet child({ props })}
													<Button
														{...props}
														variant="ghost"
														size="icon-sm"
														aria-label={`Actions for ${label.name}`}
													>
														<MoreHorizontal aria-hidden="true" />
													</Button>
												{/snippet}
											</DropdownMenu.Trigger>
											<DropdownMenu.Content align="end">
												<DropdownMenu.Item onSelect={() => openEdit(label)}>
													<Pencil aria-hidden="true" />
													Edit label
												</DropdownMenu.Item>
												<DropdownMenu.Item>
													{#snippet child({ props })}
														<a {...props} href={workspacePath(slug, `/issues?label=${label.id}`)}>
															<List aria-hidden="true" />
															See tagged issues
														</a>
													{/snippet}
												</DropdownMenu.Item>
												{#if mergeTargets(label, labels).length > 0}
													<DropdownMenu.Item onSelect={() => openMerge(label)}>
														<Merge aria-hidden="true" />
														Merge into another label
													</DropdownMenu.Item>
												{/if}
												<DropdownMenu.Separator />
												<DropdownMenu.Item
													variant="destructive"
													onSelect={() => openRemoval(label)}
												>
													<Trash2 aria-hidden="true" />
													Delete label
												</DropdownMenu.Item>
											</DropdownMenu.Content>
										</DropdownMenu.Root>
									</li>
								{/each}
							</ul>
						{/if}
					</div>
				{/each}

				<p class="text-sm leading-normal text-muted-foreground text-pretty">
					A group makes its labels mutually exclusive, so picking one clears the other. Everything
					else stacks freely. Labels appear on every issue in
					<a
						href={workspacePath(slug, "/issues")}
						class="text-link underline-offset-2 hover:text-link-hover hover:underline"
					>
						the issue board
					</a>.
				</p>
			{/if}
		</div>
	{/if}
</SettingsPage>

<MergeLabelDialog
	bind:open={mergeOpen}
	source={mergeSource}
	{labels}
	{usage}
	merging={busy}
	onconfirm={confirmMerge}
/>

<DeleteLabelDialog
	bind:open={removalOpen}
	label={removalTarget}
	{labels}
	usage={removalUsage}
	removing={busy}
	onconfirm={confirmRemoval}
	onretry={retryUsage}
	onmergeinstead={mergeInstead}
/>

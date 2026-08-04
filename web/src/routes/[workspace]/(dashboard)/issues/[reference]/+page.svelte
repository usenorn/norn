<script lang="ts">
	import { goto, invalidateAll } from "$app/navigation";
	import { page } from "$app/state";
	import ChevronDown from "@lucide/svelte/icons/chevron-down";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import Archive from "@lucide/svelte/icons/archive";
	import ArchiveRestore from "@lucide/svelte/icons/archive-restore";
	import Pencil from "@lucide/svelte/icons/pencil";
	import Trash2 from "@lucide/svelte/icons/trash-2";
	import UserRound from "@lucide/svelte/icons/user-round";
	import Users from "@lucide/svelte/icons/users";
	import List from "@lucide/svelte/icons/list";
	import Tags from "@lucide/svelte/icons/tags";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as DropdownMenu from "$lib/components/ui/dropdown-menu/index.js";
	import StatusIcon from "$lib/components/norn/status-icon.svelte";
	import Tag from "$lib/components/norn/tag.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import { api } from "$lib/api";
	import { dueLabel, onDate, onDateAndTime, overdue } from "$lib/time";
	import { renderMarkdown } from "$lib/issues/markdown";
	import {
		activityLine,
		issueFailureMessage,
		priorities,
		priorityLabel,
		readIssueFailure,
		statusLabel,
		type IssueFailure,
		type IssuePriority,
	} from "$lib/issues/issues";
	import { issueEditSchema } from "$lib/issues/issue-schema";
	import { defaults, setError, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import * as Form from "$lib/components/ui/form/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { Textarea } from "$lib/components/ui/textarea/index.js";
	import PriorityIcon from "$lib/components/norn/priority-icon.svelte";
	import {
		conflictFailure,
		labelFailureMessage,
		sectioned,
		selectable,
		toggled,
		type Label,
		type LabelFailure,
	} from "$lib/labels/labels";
	import { workspacePath } from "$lib/workspace/navigation";
	import { issueDetailPreviewStates } from "./preview";
	import type { IssueDetail } from "./+page";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV
			? issueDetailPreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined
	);

	let applied = $state<Label[] | null>(null);
	let labelFailure = $state<LabelFailure | null>(null);
	let failure = $state<IssueFailure | null>(null);
	let editing = $state(false);
	let pendingTeamId = $state("");

	const detail = $derived<IssueDetail>(preview?.detail ?? data.detail);
	const ready = $derived(detail.kind === "ready" ? detail : null);
	const issue = $derived(ready?.issue ?? null);
	const labels = $derived(applied ?? issue?.labels ?? []);
	const slug = $derived(page.params.workspace ?? "");
	const at = $derived((path: string) => workspacePath(slug, path));

	const available = $derived(
		issue ? selectable(ready?.labels ?? [], issue.teamId) : ([] as Label[])
	);
	const sections = $derived(sectioned(available, ready?.groups ?? []));

	const teams = $derived(data.teams ?? []);
	const assigneeName = $derived(
		ready?.members.find((member) => member.accountId === issue?.assigneeAccountId)?.displayName ?? ""
	);
	const described = $derived(renderMarkdown(issue?.description ?? ""));

	const form = superForm(defaults(zod4(issueEditSchema)), {
		id: "issue-edit",
		SPA: true,
		validators: zod4Client(issueEditSchema),
		resetForm: false,
		onUpdate: async ({ form: pending }) => {
			if (!pending.valid || !issue) return;

			const clear: string[] = [];
			const body: Record<string, unknown> = {};

			if (pending.data.title !== issue.title) body.title = pending.data.title;
			if (pending.data.description !== issue.description) body.description = pending.data.description;

			if (pending.data.estimate === "") {
				if (issue.estimate) clear.push("estimate");
			} else if (Number(pending.data.estimate) !== issue.estimate) {
				body.estimate = Number(pending.data.estimate);
			}

			if (pending.data.dueOn === "") {
				if (issue.dueOn) clear.push("dueOn");
			} else if (pending.data.dueOn !== issue.dueOn) {
				body.dueOn = pending.data.dueOn;
			}

			if (clear.length > 0) body.clear = clear;

			if (Object.keys(body).length === 0) {
				editing = false;

				return;
			}

			if (await patch(body)) {
				editing = false;
				announcement = "Saved.";

				return;
			}

			if (failure?.kind === "invalid") {
				for (const field of failure.fields) {
					if (field === "title") setError(pending, "title", "Give the issue a title.");
					if (field === "description") setError(pending, "description", "That description cannot be stored.");
					if (field === "estimate") setError(pending, "estimate", "Use a whole number of points.");
					if (field === "dueOn") setError(pending, "dueOn", "Use a date like 2026-09-01.");
				}
			}
		},
	});
	const { form: formData, enhance, submitting } = form;

	$effect(() => {
		if (!issue || editing) return;

		const seed = {
			title: issue.title,
			description: issue.description,
			priority: issue.priority,
			assigneeId: issue.assigneeAccountId ?? "",
			estimate: issue.estimate ? String(issue.estimate) : "",
			dueOn: issue.dueOn ?? "",
		};

		formData.update((current) => ({ ...current, ...seed }), { taint: false });
	});
	let working = $state(false);
	let announcement = $state("");

	function readFailure(error: unknown): LabelFailure {
		if (error && typeof error === "object" && "code" in error) {
			const problem = error as { code: string; issues?: number; conflicts?: string[] };
			const conflict = conflictFailure(problem.code, problem.issues, problem.conflicts);

			if (conflict) return conflict;
		}

		if (error && typeof error === "object" && "status" in error) {
			if ((error as { status: number }).status === 403) return { kind: "forbidden" };
		}

		return { kind: "unavailable" };
	}

	async function submit(labelIds: string[]) {
		if (!issue) return;

		working = true;
		labelFailure = null;
		failure = null;

		try {
			const { data: next, error } = await api.PUT(
				"/workspaces/{workspaceId}/issues/{issueId}/labels",
				{
					params: { path: { workspaceId: data.workspace.id, issueId: issue.id } },
					body: { expectedVersion: issue.version, labelIds },
				}
			);

			if (error) {
				labelFailure = readFailure(error);
				applied = null;
				await invalidateAll();

				return;
			}

			applied = next ?? [];
			announcement = `This issue now carries ${applied.length} ${applied.length === 1 ? "label" : "labels"}.`;
			await invalidateAll();
		} catch {
			labelFailure = { kind: "unavailable" };
		} finally {
			working = false;
		}
	}

	async function patch(body: Record<string, unknown>): Promise<boolean> {
		if (!issue) return false;

		working = true;
		failure = null;
		labelFailure = null;

		try {
			const { error } = await api.PATCH("/workspaces/{workspaceId}/issues/{issueId}", {
				params: { path: { workspaceId: data.workspace.id, issueId: issue.id } },
				body: { expectedVersion: issue.version, ...body },
			});

			if (error) {
				failure = readIssueFailure(error);
				await invalidateAll();

				return false;
			}

			await invalidateAll();

			return true;
		} catch {
			failure = { kind: "unavailable" };

			return false;
		} finally {
			working = false;
		}
	}

	async function move(stateId: string) {
		if (!issue || issue.state.id === stateId) return;

		await patch({ stateId });
	}

	async function setPriority(priority: IssuePriority) {
		if (!issue || issue.priority === priority) return;

		await patch({ priority });
	}

	async function setAssignee(accountId: string) {
		if (!issue) return;

		await patch(accountId === "" ? { clear: ["assignee"] } : { assigneeId: accountId });
	}

	async function setStatus(status: "active" | "archived" | "pending_deletion") {
		if (!issue) return;

		working = true;
		failure = null;

		try {
			const { error } = await api.POST("/workspaces/{workspaceId}/issues/{issueId}/status", {
				params: { path: { workspaceId: data.workspace.id, issueId: issue.id } },
				body: { expectedVersion: issue.version, status },
			});

			if (error) failure = readIssueFailure(error);

			await invalidateAll();
		} catch {
			failure = { kind: "unavailable" };
		} finally {
			working = false;
		}
	}

	async function moveToTeam(teamId: string, acknowledgeLabelLoss: boolean) {
		if (!issue || issue.teamId === teamId) return;

		working = true;
		failure = null;

		try {
			const { data: moved, error } = await api.POST(
				"/workspaces/{workspaceId}/issues/{issueId}/team",
				{
					params: { path: { workspaceId: data.workspace.id, issueId: issue.id } },
					body: { expectedVersion: issue.version, teamId, acknowledgeLabelLoss },
				}
			);

			if (error) {
				failure = readIssueFailure(error);
				pendingTeamId = failure.kind === "labels_out_of_scope" ? teamId : "";

				return;
			}

			pendingTeamId = "";

			if (moved) {
				await goto(at(`/issues/${moved.reference}`), { invalidateAll: true });
			}
		} catch {
			failure = { kind: "unavailable" };
		} finally {
			working = false;
		}
	}

	function when(timestamp: string): string {
		return onDateAndTime(timestamp, data.workspace.timezone);
	}
</script>

<svelte:head>
	<title>{issue ? `${issue.reference} · ${issue.title}` : "Issue"} · Norn</title>
</svelte:head>

<div class="flex min-h-0 flex-1 flex-col">
	<div class="flex-none border-b border-line-default">
		<div class="flex h-11 items-center gap-2 pr-3 pl-4">
			<List class="size-icon-toolbar shrink-0 text-muted-foreground" aria-hidden="true" />
			<a
				href={at("/issues")}
				class="text-md font-medium tracking-snug whitespace-nowrap text-muted-foreground transition-colors duration-110 ease-out hover:text-ink-900 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
			>
				Issues
			</a>
			{#if issue}
				<span class="text-md text-muted-foreground" aria-hidden="true">/</span>
				<span class="font-mono text-xs text-muted-foreground">{issue.reference}</span>
			{/if}
		</div>
	</div>

	<div class="flex-1 overflow-auto">
		<div
			class="mx-auto flex w-full max-w-140 flex-col gap-6 px-4 py-6 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))]"
		>
			<p class="sr-only" role="status" aria-live="polite">{announcement}</p>

			{#if detail.kind === "loading"}
				<div class="h-40 animate-pulse rounded-lg bg-paper-2" aria-busy="true"></div>
			{:else if detail.kind === "not_found"}
				<div class="flex flex-col gap-2">
					<h1 class="text-md font-medium tracking-snug text-ink-900">No issue here</h1>
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						There is no issue at this address, or it belongs to a team you are not on.
					</p>
					<div>
						<Button variant="secondary" size="sm" href={at("/issues")}>Back to issues</Button>
					</div>
				</div>
			{:else if detail.kind === "unavailable" || !issue || !ready}
				<Alert.Root variant="destructive">
					<CircleX aria-hidden="true" />
					<Alert.Title>We could not load this issue</Alert.Title>
					<Alert.Description>Nothing changed. Wait a moment and try again.</Alert.Description>
				</Alert.Root>
			{:else}
				{#if labelFailure}
					<Alert.Root variant="destructive">
						<CircleX aria-hidden="true" />
						<Alert.Title>That did not work</Alert.Title>
						<Alert.Description>{labelFailureMessage(labelFailure)}</Alert.Description>
					</Alert.Root>
				{/if}

				{#if failure}
					<Alert.Root variant={failure.kind === "labels_out_of_scope" ? "default" : "destructive"}>
						<CircleX aria-hidden="true" />
						<Alert.Title>
							{failure.kind === "stale"
								? "Someone got there first"
								: failure.kind === "labels_out_of_scope"
									? "This move drops labels"
									: "That did not work"}
						</Alert.Title>
						<Alert.Description>
							{issueFailureMessage(failure)}
							{#if failure.kind === "labels_out_of_scope" && pendingTeamId}
								<span class="mt-2 block">
									<Button
										variant="secondary"
										size="sm"
										disabled={working}
										onclick={() => moveToTeam(pendingTeamId, true)}
									>
										Move anyway
									</Button>
								</span>
							{/if}
						</Alert.Description>
					</Alert.Root>
				{/if}

				{#if issue.status !== "active"}
					<Alert.Root>
						<Archive aria-hidden="true" />
						<Alert.Title>
							{issue.status === "archived" ? "Archived" : "Deleted"}
						</Alert.Title>
						<Alert.Description>
							{issue.status === "archived"
								? `Archived ${issue.archivedAt ? when(issue.archivedAt) : ""}. It keeps its reference and stays readable.`
								: "This issue is scheduled to be removed for good. Restore it to keep it."}
						</Alert.Description>
					</Alert.Root>
				{/if}

				{#if editing}
					<form method="POST" use:enhance class="flex flex-col gap-4">
						<Form.Field {form} name="title">
							<Form.Control>
								{#snippet children({ props })}
									<Form.Label>Title</Form.Label>
									<Input {...props} bind:value={$formData.title} disabled={$submitting} />
								{/snippet}
							</Form.Control>
							<Form.FieldErrors />
						</Form.Field>

						<Form.Field {form} name="description">
							<Form.Control>
								{#snippet children({ props })}
									<Form.Label>Description</Form.Label>
									<Textarea
										{...props}
										bind:value={$formData.description}
										rows={10}
										disabled={$submitting}
										class="font-mono text-sm"
									/>
								{/snippet}
							</Form.Control>
							<Form.Description>Markdown. Stored exactly as you type it.</Form.Description>
							<Form.FieldErrors />
						</Form.Field>

						<div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
							<Form.Field {form} name="estimate">
								<Form.Control>
									{#snippet children({ props })}
										<Form.Label>Estimate</Form.Label>
										<Input
											{...props}
											bind:value={$formData.estimate}
											inputmode="numeric"
											placeholder="Points"
											disabled={$submitting}
										/>
									{/snippet}
								</Form.Control>
								<Form.FieldErrors />
							</Form.Field>

							<Form.Field {form} name="dueOn">
								<Form.Control>
									{#snippet children({ props })}
										<Form.Label>Due date</Form.Label>
										<Input
											{...props}
											type="date"
											bind:value={$formData.dueOn}
											disabled={$submitting}
										/>
									{/snippet}
								</Form.Control>
								<Form.FieldErrors />
							</Form.Field>
						</div>

						<div class="flex items-center gap-2">
							<Form.Button disabled={$submitting || working}>
								{$submitting ? "Saving" : "Save"}
							</Form.Button>
							<Button
								type="button"
								variant="ghost"
								size="sm"
								disabled={$submitting}
								onclick={() => (editing = false)}
							>
								Cancel
							</Button>
						</div>
					</form>
				{:else}
					<div class="flex flex-col gap-2">
						<div class="flex items-start justify-between gap-3">
							<span class="font-mono text-xs text-muted-foreground">{issue.reference}</span>
							<Button
								variant="ghost"
								size="sm"
								disabled={working}
								onclick={() => (editing = true)}
							>
								<Pencil aria-hidden="true" />
								Edit
							</Button>
						</div>
						<h1 class="text-lg font-medium tracking-snug text-ink-900 text-pretty">
							{issue.title}
						</h1>
					</div>

					<section class="flex flex-col gap-2">
						<h2 class="text-sm font-medium text-ink-900">Description</h2>
						{#if described}
							<div
								class="prose prose-sm max-w-none text-ink-900 prose-headings:font-medium prose-headings:tracking-snug prose-a:text-link prose-pre:bg-paper-2 prose-code:font-mono"
							>
								<!-- eslint-disable-next-line svelte/no-at-html-tags -->
								{@html described}
							</div>
						{:else}
							<p class="text-sm text-muted-foreground">No description yet.</p>
						{/if}
					</section>
				{/if}

				<section class="flex flex-col gap-2">
					<h2 class="text-sm font-medium text-ink-900">State</h2>
					<div>
						<DropdownMenu.Root>
							<DropdownMenu.Trigger>
								{#snippet child({ props })}
									<Button {...props} variant="outline" size="sm" disabled={working}>
										<StatusIcon category={issue.state.category} decorative />
										{issue.state.name}
										<ChevronDown aria-hidden="true" />
									</Button>
								{/snippet}
							</DropdownMenu.Trigger>
							<DropdownMenu.Content align="start">
								<DropdownMenu.RadioGroup
									value={issue.state.id}
									onValueChange={(value) => move(value)}
								>
									<DropdownMenu.GroupHeading>Move to</DropdownMenu.GroupHeading>
									{#each ready.states as state (state.id)}
										<DropdownMenu.RadioItem value={state.id}>
											{state.name}
										</DropdownMenu.RadioItem>
									{/each}
								</DropdownMenu.RadioGroup>
							</DropdownMenu.Content>
						</DropdownMenu.Root>
					</div>
				</section>

				<section class="flex flex-col gap-2">
					<h2 class="text-sm font-medium text-ink-900">Priority</h2>
					<div>
						<DropdownMenu.Root>
							<DropdownMenu.Trigger>
								{#snippet child({ props })}
									<Button {...props} variant="outline" size="sm" disabled={working}>
										<PriorityIcon priority={issue.priority} />
										{priorityLabel(issue.priority)}
										<ChevronDown aria-hidden="true" />
									</Button>
								{/snippet}
							</DropdownMenu.Trigger>
							<DropdownMenu.Content align="start">
								<DropdownMenu.RadioGroup
									value={issue.priority}
									onValueChange={(value) => setPriority(value as IssuePriority)}
								>
									{#each priorities as choice (choice.value)}
										<DropdownMenu.RadioItem value={choice.value}>
											{choice.label}
										</DropdownMenu.RadioItem>
									{/each}
								</DropdownMenu.RadioGroup>
							</DropdownMenu.Content>
						</DropdownMenu.Root>
					</div>
				</section>

				<section class="flex flex-col gap-2">
					<h2 class="text-sm font-medium text-ink-900">Assignee</h2>
					<div>
						<DropdownMenu.Root>
							<DropdownMenu.Trigger>
								{#snippet child({ props })}
									<Button {...props} variant="outline" size="sm" disabled={working}>
										<UserRound aria-hidden="true" />
										{assigneeName || "Unassigned"}
										<ChevronDown aria-hidden="true" />
									</Button>
								{/snippet}
							</DropdownMenu.Trigger>
							<DropdownMenu.Content align="start" class="max-h-80 overflow-auto">
								<DropdownMenu.RadioGroup
									value={issue.assigneeAccountId ?? ""}
									onValueChange={(value) => setAssignee(value)}
								>
									<DropdownMenu.RadioItem value="">Unassigned</DropdownMenu.RadioItem>
									<DropdownMenu.Separator />
									{#each ready.members as member (member.accountId)}
										<DropdownMenu.RadioItem value={member.accountId}>
											{member.displayName || member.email}
										</DropdownMenu.RadioItem>
									{/each}
								</DropdownMenu.RadioGroup>
							</DropdownMenu.Content>
						</DropdownMenu.Root>
					</div>
				</section>

				<section class="flex flex-col gap-2">
					<h2 class="text-sm font-medium text-ink-900">Estimate and due date</h2>
					<dl class="flex flex-wrap gap-x-8 gap-y-2 text-sm">
						<div class="flex items-baseline gap-2">
							<dt class="text-muted-foreground">Estimate</dt>
							<dd class="font-mono text-ink-900">
								{issue.estimate ? `${issue.estimate} ${issue.estimate === 1 ? "point" : "points"}` : "None"}
							</dd>
						</div>
						<div class="flex items-baseline gap-2">
							<dt class="text-muted-foreground">Due</dt>
							<dd
								class="font-mono {issue.dueOn && overdue(issue.dueOn, data.now)
									? 'text-priority-urgent'
									: 'text-ink-900'}"
							>
								{issue.dueOn ? dueLabel(issue.dueOn, data.now, data.workspace.timezone) : "No date"}
							</dd>
						</div>
					</dl>
				</section>

				<section class="flex flex-col gap-2">
					<h2 class="text-sm font-medium text-ink-900">Team</h2>
					<div>
						<DropdownMenu.Root>
							<DropdownMenu.Trigger>
								{#snippet child({ props })}
									<Button {...props} variant="outline" size="sm" disabled={working}>
										<Users aria-hidden="true" />
										{issue.teamKey}
										<ChevronDown aria-hidden="true" />
									</Button>
								{/snippet}
							</DropdownMenu.Trigger>
							<DropdownMenu.Content align="start">
								<DropdownMenu.RadioGroup
									value={issue.teamId}
									onValueChange={(value) => moveToTeam(value, false)}
								>
									<DropdownMenu.GroupHeading>Move to</DropdownMenu.GroupHeading>
									{#each teams as team (team.id)}
										<DropdownMenu.RadioItem value={team.id}>
											{team.key} · {team.name}
										</DropdownMenu.RadioItem>
									{/each}
								</DropdownMenu.RadioGroup>
							</DropdownMenu.Content>
						</DropdownMenu.Root>
					</div>
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						{issue.reference} keeps its reference wherever it goes. Its state becomes the matching one
						on the new team.
					</p>
				</section>

				<section class="flex flex-col gap-2">
					<h2 class="text-sm font-medium text-ink-900">Labels</h2>

					<div class="flex flex-wrap items-center gap-2">
						{#each labels as label (label.id)}
							<Tag name={label.name} color={label.color} />
						{/each}

						{#if labels.length === 0}
							<span class="text-sm text-muted-foreground">None yet.</span>
						{/if}

						<DropdownMenu.Root>
							<DropdownMenu.Trigger>
								{#snippet child({ props })}
									<Button {...props} variant="outline" size="sm" disabled={working}>
										<Tags aria-hidden="true" />
										{working ? "Saving" : "Labels"}
									</Button>
								{/snippet}
							</DropdownMenu.Trigger>
							<DropdownMenu.Content align="start" class="max-h-80 overflow-auto">
								{#if available.length === 0}
									<DropdownMenu.Label>No labels apply to this team yet</DropdownMenu.Label>
								{:else}
									{#each sections as section (section.group?.id ?? "ungrouped")}
										{#if section.group}
											<DropdownMenu.RadioGroup
												value={labels.find((label) => label.groupId === section.group!.id)?.id ??
													""}
												onValueChange={(value) => {
													const chosen = section.labels.find((label) => label.id === value);
													if (chosen) submit(toggled(labels, chosen));
												}}
											>
												<DropdownMenu.GroupHeading>
													{section.group.name} · one only
												</DropdownMenu.GroupHeading>
												{#each section.labels as label (label.id)}
													<DropdownMenu.RadioItem value={label.id}>
														{label.name}
													</DropdownMenu.RadioItem>
												{/each}
											</DropdownMenu.RadioGroup>
											<DropdownMenu.Separator />
										{:else if section.labels.length > 0}
											<DropdownMenu.Group>
												<DropdownMenu.GroupHeading>Any number</DropdownMenu.GroupHeading>
												{#each section.labels as label (label.id)}
													<DropdownMenu.CheckboxItem
														checked={labels.some((chosen) => chosen.id === label.id)}
														onCheckedChange={() => submit(toggled(labels, label))}
													>
														{label.name}
													</DropdownMenu.CheckboxItem>
												{/each}
											</DropdownMenu.Group>
										{/if}
									{/each}
								{/if}
							</DropdownMenu.Content>
						</DropdownMenu.Root>
					</div>

					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						Labels are defined in
						<a
							href={at("/settings/labels")}
							class="text-link underline-offset-2 hover:text-link-hover hover:underline"
						>
							workspace settings
						</a>. A label narrowed to another team does not appear here.
					</p>
				</section>

				<section class="flex flex-col gap-2">
					<h2 class="text-sm font-medium text-ink-900">History</h2>
					<dl class="flex flex-col gap-1 text-sm">
						<div class="flex flex-wrap items-baseline gap-x-2">
							<dt class="text-muted-foreground">Raised</dt>
							<dd class="font-mono text-xs text-ink-900">
								<time datetime={issue.createdAt}>{when(issue.createdAt)}</time>
							</dd>
						</div>
						<div class="flex flex-wrap items-baseline gap-x-2">
							<dt class="text-muted-foreground">In {issue.state.name} since</dt>
							<dd class="font-mono text-xs text-ink-900">
								<time datetime={issue.stateEnteredAt}>{when(issue.stateEnteredAt)}</time>
							</dd>
						</div>
						{#if issue.completedAt}
							<div class="flex flex-wrap items-baseline gap-x-2">
								<dt class="text-muted-foreground">Finished</dt>
								<dd class="font-mono text-xs text-ink-900">
									<time datetime={issue.completedAt}>{when(issue.completedAt)}</time>
								</dd>
							</div>
						{/if}
					</dl>
				</section>

				<section class="flex flex-col gap-2">
					<h2 class="text-sm font-medium text-ink-900">This issue</h2>
					<div class="flex flex-wrap items-center gap-2">
						{#if issue.status === "active"}
							<Button
								variant="outline"
								size="sm"
								disabled={working}
								onclick={() => setStatus("archived")}
							>
								<Archive aria-hidden="true" />
								Archive
							</Button>
						{:else}
							<Button
								variant="outline"
								size="sm"
								disabled={working}
								onclick={() => setStatus("active")}
							>
								<ArchiveRestore aria-hidden="true" />
								{issue.status === "archived" ? "Take out of the archive" : "Restore"}
							</Button>
						{/if}

						{#if issue.status !== "pending_deletion"}
							<Button
								variant="ghost"
								size="sm"
								disabled={working}
								onclick={() => setStatus("pending_deletion")}
							>
								<Trash2 aria-hidden="true" />
								Delete
							</Button>
						{/if}
					</div>
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						An archived issue is kept for good and stays readable. A deleted one is removed for
						good after 30 days, and can be restored until then. Neither ever frees its reference.
					</p>
				</section>

				<section class="flex flex-col gap-2">
					<h2 class="text-sm font-medium text-ink-900">Activity</h2>

					{#if ready.activity.length === 0}
						<p class="text-sm text-muted-foreground">Nothing has happened yet.</p>
					{:else}
						<ol class="flex flex-col gap-2">
							{#each ready.activity as entry (entry.id)}
								<li class="flex flex-wrap items-baseline gap-x-2 gap-y-0.5 text-sm">
									<span class="text-ink-900">{activityLine(entry)}</span>
									{#if entry.actorName}
										<span class="text-muted-foreground">by {entry.actorName}</span>
									{/if}
									<span class="font-mono text-xs text-muted-foreground">
										<time datetime={entry.createdAt}>{when(entry.createdAt)}</time>
									</span>
								</li>
							{/each}
						</ol>
					{/if}
				</section>
			{/if}
		</div>
	</div>
</div>

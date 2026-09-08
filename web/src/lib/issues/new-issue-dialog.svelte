<script lang="ts">
	import { tick } from "svelte";
	import CalendarDays from "@lucide/svelte/icons/calendar-days";
	import ChevronDown from "@lucide/svelte/icons/chevron-down";
	import ChevronRight from "@lucide/svelte/icons/chevron-right";
	import Paperclip from "@lucide/svelte/icons/paperclip";
	import Plus from "@lucide/svelte/icons/plus";
	import Tags from "@lucide/svelte/icons/tags";
	import X from "@lucide/svelte/icons/x";
	import { defaults, setError, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import { api } from "$lib/api";
	import { showToast } from "$lib/toast/toasts";
	import * as Avatar from "$lib/components/ui/avatar/index.js";
	import * as Command from "$lib/components/ui/command/index.js";
	import * as Dialog from "$lib/components/ui/dialog/index.js";
	import * as Form from "$lib/components/ui/form/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { Switch } from "$lib/components/ui/switch/index.js";
	import Kbd from "$lib/components/norn/kbd.svelte";
	import PriorityIcon from "$lib/components/norn/priority-icon.svelte";
	import StatusIcon from "$lib/components/norn/status-icon.svelte";
	import TeamKey from "$lib/components/norn/team-key.svelte";
	import LabelDot from "$lib/labels/label-dot.svelte";
	import { initialsOf } from "$lib/team/members";
	import { onCalendarDate } from "$lib/time";
	import { formatBytes, type Attachment } from "$lib/attachments/attachments";
	import AttachmentPicker from "$lib/attachments/attachment-picker.svelte";
	import UploadList from "$lib/attachments/upload-list.svelte";
	import type { UploadTask } from "$lib/attachments/upload";
	import { assignable, type AccountKind } from "$lib/workspace/members";
	import { conflictFailure, labelColors, labelFailureMessage } from "$lib/labels/labels";
	import type { Label } from "$lib/labels/labels";
	import {
		attachFailureMessage,
		attachPending,
		describeFailureMessage,
		describedWith,
		markdownFor,
		pendingFrom,
		type PendingFile,
	} from "./new-issue-attachments";
	import type { Project } from "$lib/projects/projects";
	import type { WorkflowState } from "$lib/team/states";
	import type { Team } from "$lib/team/teams";
	import PropertyPicker, { type PickerOption } from "./property-picker.svelte";
	import { duePresets } from "./facets";
	import { newIssueSchema, type NewIssueInput } from "./new-issue-schema";
	import { draftIssue, type CreationOutcome } from "./creating";
	import DescriptionEditor from "$lib/issues/description-editor.svelte";
	import { issueFailureMessage, priorities, priorityLabel, readIssueFailure } from "./issues";
	import type { Issue } from "./issues";

	let {
		open = $bindable(false),
		workspaceId,
		workspace,
		teams,
		states,
		members,
		labels,
		projects,
		today,
		now,
		prefill,
		onraising,
		onsettled,
	}: {
		open?: boolean;
		workspaceId: string;
		workspace: string;
		teams: Team[];
		states: Record<string, WorkflowState[]>;
		members: { accountId: string; displayName?: string; kind?: AccountKind }[];
		labels: Label[];
		projects: Project[];
		today: string;
		now: string;
		prefill?: Partial<NewIssueInput>;
		onraising?: (key: string, draft: Issue) => void;
		onsettled?: (outcome: CreationOutcome) => void | Promise<void>;
	} = $props();

	let failure = $state<string | null>(null);
	let wasOpen = $state(false);
	let teamStates = $state.raw<Record<string, WorkflowState[]>>({});
	let titleField = $state<HTMLInputElement | null>(null);
	let fields = $state<HTMLFormElement | null>(null);
	let attaching = $state.raw<PendingFile[]>([]);
	let uploads = $state.raw<UploadTask[]>([]);
	let dragging = $state(false);
	let unconfirmed = $state(false);
	let opening = $state(0);

	const unknownCreate =
		"We could not tell whether the issue was created. Check the issue list before trying again.";
	let aborts = new Map<string, () => void>();

	type Submission = {
		key: string;
		opening: number;
		workspaceId: string;
		consumer: ((outcome: CreationOutcome) => void | Promise<void>) | undefined;
		files: PendingFile[];
		description: string;
		issue: Issue | null;
		attached: Attachment[];
		settled: boolean;
		running: boolean;
		unresolved: boolean;
	};

	let live = $state.raw<Submission | null>(null);
	let holding = $state<{ issue: Issue | null; running: boolean }>({ issue: null, running: false });

	const raised = $derived(holding.issue);

	function settle(own: Submission) {
		if (own.settled || !own.issue) return;

		own.settled = true;
		announce(own, { key: own.key, kind: "created", issue: own.issue });
	}

	function announce(own: Submission, outcome: CreationOutcome) {
		try {
			const handled = own.consumer?.(outcome);

			if (handled) void handled.catch((reason: unknown) => reportConsumer(own, reason));
		} catch (reason) {
			reportConsumer(own, reason);
		}
	}

	function reportConsumer(own: Submission, _reason: unknown) {
		const reference = own.issue?.reference;

		showToast(
			reference
				? `${reference} was created, but what should have happened next did not.`
				: "The issue was created, but what should have happened next did not."
		);
	}

	function abandon() {
		const own = live;

		live = null;
		holding = { issue: null, running: false };

		if (!own) return;

		if (!own.unresolved) {
			settle(own);

			return;
		}

		void currentIssue(own).then((current) => {
			if (current) {
				own.issue = current;
				own.unresolved = false;
				settle(own);

				return;
			}

			showToast(
				own.issue
					? `${own.issue.reference} was created and its files are attached, but we could not ` +
							"confirm the rest. Open it to check."
					: "The issue was created, but we could not confirm the rest."
			);
		});
	}
	let coined = $state.raw<Label[]>([]);
	let labelSearch = $state("");
	let coining = $state(false);
	let labelFailure = $state<string | null>(null);
	let resuming = $state(false);
	let intake = 0;

	function takeFiles(files: File[]) {
		if (files.length === 0 || busy) return;

		setPending([...attaching, ...pendingFrom(files, () => `${(intake += 1)}`)]);
	}

	function setPending(next: PendingFile[]) {
		attaching = next;

		if (live && !holding.running) live.files = next;
	}

	function carriesFiles(event: DragEvent): boolean {
		return Array.from(event.dataTransfer?.types ?? []).includes("Files");
	}

	function dragEnter(event: DragEvent) {
		if (!carriesFiles(event)) return;

		event.preventDefault();
		dragging = !busy;
	}

	function dragLeave(event: DragEvent) {
		if (event.currentTarget === event.target) dragging = false;
	}

	function dropFiles(event: DragEvent) {
		if (!carriesFiles(event)) return;

		event.preventDefault();
		dragging = false;

		takeFiles(Array.from(event.dataTransfer?.files ?? []));
	}

	function dropPending(key: string) {
		setPending(attaching.filter((file) => file.key !== key));
	}

	async function coinLabel(name: string) {
		const wanted = name.trim();

		if (!wanted || coining) return;

		coining = true;
		labelFailure = null;

		const { data, error } = await api.POST("/workspaces/{workspaceId}/labels", {
			params: { path: { workspaceId } },
			body: { name: wanted, color: labelColors[(labels.length + coined.length) % labelColors.length] },
		});

		coining = false;

		if (error || !data) {
			const conflict =
				error && typeof error === "object" && "code" in error
					? conflictFailure(String(error.code))
					: null;

			labelFailure = labelFailureMessage(conflict ?? { kind: "unavailable" });

			return;
		}

		coined = [...coined, data];
		labelSearch = "";
		toggleLabel(data.id);
	}

	const form = superForm(defaults(zod4(newIssueSchema)), {
		id: "new-issue",
		SPA: true,
		validators: zod4Client(newIssueSchema),
		resetForm: false,
		onUpdate: async ({ form: pending, cancel }) => {
			if (!pending.valid) return;

			if (unconfirmed) {
				cancel();

				return;
			}

			const own: Submission = live ?? {
				key: crypto.randomUUID(),
				opening,
				workspaceId,
				consumer: onsettled,
				files: attaching,
				description: pending.data.description,
				issue: null,
				attached: [],
				settled: false,
				running: false,
				unresolved: false,
			};

			const mine = () => live === own;
			const showing = () => own.opening === opening;

			own.running = true;
			live = own;
			holding = { issue: own.issue, running: true };
			failure = null;

			if (!own.issue) {
				const filing = openState ?? available.find((state) => state.isDefault);
				const draft =
					team && filing
						? draftIssue(own.key, pending.data, {
								workspaceId,
								team,
								state: filing,
								labels: known,
								projects,
								now,
							})
						: undefined;
				const detached = Boolean(draft) && own.files.length === 0 && !pending.data.createMore;

				if (draft) onraising?.(own.key, draft);

				if (detached) {
					live = null;
					open = false;
				}

				let created: Issue | undefined;
				let error: unknown;

				try {
					const raisedIssue = await api.POST("/workspaces/{workspaceId}/issues", {
						params: { path: { workspaceId: own.workspaceId } },
						body: {
							teamId: pending.data.teamId,
							title: pending.data.title,
							description: pending.data.description || undefined,
							priority: pending.data.priority,
							stateId: pending.data.stateId || undefined,
							assigneeId: pending.data.assigneeId || undefined,
							projectId: pending.data.projectId || undefined,
							cycleId: pending.data.cycleId || undefined,
							labelIds: pending.data.labelIds.length > 0 ? pending.data.labelIds : undefined,
							dueOn: pending.data.dueOn || undefined,
						},
					});

					created = raisedIssue.data;
					error = raisedIssue.error;
				} catch {
					own.running = false;
					own.settled = true;

					announce(own, { key: own.key, kind: "refused", failure: unknownCreate });

					if (showing()) {
						holding = { issue: null, running: false };
						unconfirmed = true;
						failure = unknownCreate;
					}

					if (!mine()) {
						cancel();

						return;
					}

					live = null;

					return;
				}

				own.running = false;

				if (error || !created) {
					const read = readIssueFailure(error);
					const refusal = issueFailureMessage(read);

					own.settled = true;
					announce(own, {
						key: own.key,
						kind: "refused",
						failure: refusal,
						...(detached && showing() ? { input: pending.data } : {}),
					});

					if (!mine()) {
						cancel();

						return;
					}

					live = null;
					holding = { issue: null, running: false };

					if (read.kind === "invalid") {
						for (const field of read.fields) {
							if (field === "title") setError(pending, "title", "Give the issue a title.");
							if (field === "dueOn") setError(pending, "dueOn", "Use a date like 2026-09-01.");
						}
					}

					failure = refusal;

					return;
				}

				own.issue = created;
				own.running = true;

				if (mine()) holding = { issue: created, running: true };
			}

			if (own.files.length > 0) {
				const outcome = await attachPending(
					own.workspaceId,
					own.issue.id,
					own.files,
					(tasks) => {
						if (mine()) uploads = tasks;
					},
					(key, abort) => aborts.set(key, abort)
				);

				aborts.clear();
				own.attached = [...own.attached, ...outcome.attached];
				own.files = [...outcome.failed, ...outcome.cancelled];

				if (mine()) {
					attaching = own.files;
					uploads = [];
				}
			}

			if (own.attached.length > 0 && own.files.length === 0) {
				const saved = await describeIssue(own);

				if (!saved) {
					const unresolved = describeUncertain;

					own.running = false;
					own.unresolved = unresolved;

					if (!unresolved) settle(own);

					if (!mine()) {
						cancel();

						return;
					}

					holding = { issue: own.issue, running: false };
					failure = unresolved
						? describeFailureMessage("uncertain")
						: describeFailureMessage(describeConflict ? "conflict" : "unavailable");

					return;
				}

				own.issue = saved;
				own.attached = [];
				own.unresolved = false;
			}

			own.running = false;

			if (mine()) holding = { issue: own.issue, running: false };

			if (own.files.length > 0) {
				if (!mine()) {
					cancel();

					return;
				}

				failure = attachFailureMessage(own.files);

				return;
			}

			settle(own);

			if (!mine()) {
				cancel();

				return;
			}

			live = null;
			holding = { issue: null, running: false };

			if (!pending.data.createMore) {
				open = false;

				return;
			}

			attaching = [];
			uploads = [];
			pending.data.title = "";
			pending.data.description = "";

			resuming = true;
		},
	});

	let describeConflict = false;
	let describeUncertain = false;

	async function describeIssue(own: Submission): Promise<Issue | null> {
		if (!own.issue) return null;

		describeConflict = false;
		describeUncertain = false;

		const links = markdownFor(own.attached);
		const saved = await saveDescription(own, own.issue.version, describedWith(own.description, links));

		if (saved) return saved;

		if (!describeConflict && !describeUncertain) return null;

		const current = await currentIssue(own);

		if (!current) return null;

		if (current.description.includes(links)) {
			describeConflict = false;
			describeUncertain = false;

			return current;
		}

		if (describeUncertain) {
			own.issue = current;
			describeUncertain = false;
			describeConflict = false;
		}

		return saveDescription(own, current.version, describedWith(current.description, links));
	}

	async function saveDescription(
		own: Submission,
		expectedVersion: number,
		description: string
	): Promise<Issue | null> {
		if (!own.issue) return null;

		try {
			const described = await api.PATCH("/workspaces/{workspaceId}/issues/{issueId}", {
				params: { path: { workspaceId: own.workspaceId, issueId: own.issue.id } },
				body: { expectedVersion, description },
			});

			if (described.data && "id" in described.data) return described.data;

			describeConflict = readIssueFailure(described.error).kind === "stale";

			return null;
		} catch {
			describeUncertain = true;

			return null;
		}
	}

	async function currentIssue(own: Submission): Promise<Issue | null> {
		if (!own.issue) return null;

		try {
			const read = await api.GET("/workspaces/{workspaceId}/issues/{issueId}", {
				params: { path: { workspaceId: own.workspaceId, issueId: own.issue.id } },
			});

			return read.data ?? null;
		} catch {
			return null;
		}
	}

	const { form: formData, enhance, submitting } = form;

	const busy = $derived(holding.running || $submitting);

	$effect(() => {
		if (open || busy) return;

		abandon();
	});

	const team = $derived(teams.find((candidate) => candidate.id === $formData.teamId) ?? teams[0]);

	const available = $derived(team ? (states[team.id] ?? teamStates[team.id] ?? []) : []);

	const openState = $derived(available.find((state) => state.id === $formData.stateId));

	const people = $derived(members.filter((member) => assignable(member.kind)));

	const assignee = $derived(
		people.find((member) => member.accountId === $formData.assigneeId)?.displayName ?? ""
	);

	const project = $derived(projects.find((candidate) => candidate.id === $formData.projectId));

	const known = $derived([...labels, ...coined]);

	const chosenLabels = $derived(known.filter((label) => $formData.labelIds.includes(label.id)));

	const reachable = $derived(known.filter((label) => !label.teamId || label.teamId === team?.id));

	const coinable = $derived(
		labelSearch.trim().length > 0 &&
			!reachable.some(
				(label) => label.name.toLowerCase() === labelSearch.trim().toLowerCase()
			)
	);

	$effect(() => {
		const justOpened = open && !wasOpen;

		wasOpen = open;

		if (!justOpened) return;

		form.reset({ keepMessage: false });
		opening += 1;
		abandon();
		failure = null;
		unconfirmed = false;
		attaching = [];
		uploads = [];
		dragging = false;
		aborts.clear();
		coined = [];
		labelSearch = "";
		labelFailure = null;

		formData.update(
			(current) => ({
				...current,
				title: prefill?.title ?? "",
				description: prefill?.description ?? "",
				labelIds: prefill?.labelIds ?? [],
				dueOn: prefill?.dueOn ?? "",
				createMore: false,
				teamId: prefill?.teamId || teams[0]?.id || "",
				stateId: prefill?.stateId ?? "",
				priority: prefill?.priority ?? "none",
				assigneeId: prefill?.assigneeId ?? "",
				projectId: prefill?.projectId ?? "",
				cycleId: prefill?.cycleId ?? "",
			}),
			{ taint: false }
		);
	});

	$effect(() => {
		if (!open || !team) return;

		void loadStates(team.id);
	});

	$effect(() => {
		if (!resuming || $submitting || !titleField) return;

		titleField.focus();
		resuming = false;
	});

	async function loadStates(teamId: string) {
		if (states[teamId] || teamStates[teamId]) return;

		const { data } = await api.GET("/workspaces/{workspaceId}/teams/{teamId}/states", {
			params: { path: { workspaceId, teamId } },
		});

		if (data) teamStates = { ...teamStates, [teamId]: data };
	}

	function chooseTeam(teamId: string) {
		$formData.teamId = teamId;
		$formData.stateId = "";
		$formData.labelIds = [];
		$formData.cycleId = "";
		void loadStates(teamId);
	}

	function submitOnMeta(event: KeyboardEvent & { currentTarget: HTMLElement }) {
		if (unconfirmed || busy) return;

		if (!(event.metaKey || event.ctrlKey) || event.key !== "Enter") return;

		event.preventDefault();
		event.currentTarget.closest("form")?.requestSubmit();
	}

	function submitForm() {
		if (unconfirmed || busy) return;

		fields?.requestSubmit();
	}

	function toggleLabel(labelId: string) {
		$formData.labelIds = $formData.labelIds.includes(labelId)
			? $formData.labelIds.filter((held) => held !== labelId)
			: [...$formData.labelIds, labelId];
	}

	const teamOptions = $derived<PickerOption[]>(
		teams.map((candidate) => ({
			value: candidate.id,
			label: candidate.name,
			checked: candidate.id === team?.id,
		}))
	);

	const stateOptions = $derived<PickerOption[]>(
		available.map((state) => ({
			value: state.id,
			label: state.name,
			checked: state.id === $formData.stateId,
		}))
	);

	const priorityOptions = $derived<PickerOption[]>(
		priorities.map((entry) => ({
			value: entry.value,
			label: entry.label,
			checked: entry.value === $formData.priority,
		}))
	);

	const assigneeOptions = $derived<PickerOption[]>([
		...people.map((member) => ({
			value: member.accountId,
			label: member.displayName ?? "Someone",
			checked: member.accountId === $formData.assigneeId,
		})),
		{ value: "", label: "Unassigned", checked: $formData.assigneeId === "" },
	]);

	const projectOptions = $derived<PickerOption[]>([
		...projects.map((candidate) => ({
			value: candidate.id,
			label: candidate.name,
			checked: candidate.id === $formData.projectId,
		})),
		{ value: "", label: "No project", checked: $formData.projectId === "" },
	]);

	const labelOptions = $derived<PickerOption[]>(
		reachable.map((label) => ({
			value: label.id,
			label: label.name,
			checked: $formData.labelIds.includes(label.id),
		}))
	);

	const dueOptions = $derived<PickerOption[]>(
		duePresets(today).map((preset) => ({
			value: preset.value,
			label: preset.label,
			hint: preset.hint,
			checked: preset.value === $formData.dueOn,
		}))
	);

	const chipClass =
		"h-control-sm gap-1.5 px-2 text-sm font-normal text-foreground data-[state=open]:border-ink-400";
</script>

<Dialog.Root bind:open={() => open, (next) => (open = next || !busy ? next : open)}>
	<Dialog.Content
		class="top-21 grid-rows-[minmax(0,1fr)] max-h-[calc(100dvh-7.5rem)] overflow-hidden p-0 sm:max-w-162"
		showCloseButton={false}
	>
		<Dialog.Description class="sr-only">
			Raise an issue in a team you can see, and set its properties before it is created.
		</Dialog.Description>

		<form
			bind:this={fields}
			method="POST"
			use:enhance
			class="relative flex min-h-0 w-full min-w-0 flex-col"
			ondragenter={dragEnter}
			ondragover={dragEnter}
			ondragleave={dragLeave}
			ondrop={dropFiles}
		>
			{#if dragging}
				<div
					class="pointer-events-none absolute inset-2 z-10 flex items-center justify-center rounded-lg border-2 border-dashed border-primary bg-paper-0/85 text-md text-ink-900"
				>
					Drop files to attach them
				</div>
			{/if}
			<div
				class="flex flex-none items-center gap-2 border-b border-line-subtle py-2.75 pr-2.5 pl-3.5"
			>
				<PropertyPicker
					options={teamOptions}
					placeholder="Move to team…"
					onpick={chooseTeam}
					class="w-51.5"
				>
					{#snippet trigger(props)}
						<Button
							{...props}
							variant="ghost"
							size="sm"
							disabled={busy || Boolean(raised)}
							class="gap-1.5 px-1.5 font-medium"
						>
							{#if team}
								<TeamKey key={team.key} />
								{team.name}
							{:else}
								Choose a team
							{/if}
							<ChevronDown class="text-muted-foreground" aria-hidden="true" />
						</Button>
					{/snippet}
				</PropertyPicker>

				<ChevronRight class="size-3.25 text-muted-foreground" aria-hidden="true" />
				<Dialog.Title class="text-md font-medium text-muted-foreground">New issue</Dialog.Title>
				<span class="flex-1"></span>
				{#if team}
					<span
						class="font-mono text-xs text-muted-foreground"
						title="This issue will be numbered in {team.name}"
					>
						{team.key}
					</span>
				{/if}
				<Button
					type="button"
					variant="ghost"
					size="icon-sm"
					aria-label="Close"
					disabled={busy}
					onclick={() => (open = false)}
				>
					<X aria-hidden="true" />
				</Button>
			</div>

			<div class="min-h-0 flex-1 overflow-y-auto">
				<div class="flex flex-col gap-2.5 px-4 pt-4 pb-3">
					<Form.Field {form} name="title">
						<Form.Control>
							{#snippet children({ props })}
								<Form.Label class="sr-only">Issue title</Form.Label>
								<Input
									{...props}
									bind:ref={titleField}
									bind:value={$formData.title}
									variant="seamless"
									placeholder="Issue title"
									disabled={busy || Boolean(raised)}
									onkeydown={submitOnMeta}
									class="h-auto p-0 text-xl font-medium tracking-snug"
								/>
							{/snippet}
						</Form.Control>
						<Form.FieldErrors />
					</Form.Field>

					<Form.Field {form} name="description">
						<Form.Control>
							{#snippet children({ props })}
								<Form.Label class="sr-only">Description</Form.Label>
								<DescriptionEditor
									{...props}
									bind:value={$formData.description}
									{workspaceId}
									{workspace}
									{members}
									{teams}
									disabled={busy || Boolean(raised)}
									onfiles={takeFiles}
									onmetaenter={submitForm}
									placeholder="Add description… What is broken, what should happen instead."
								/>
							{/snippet}
						</Form.Control>
						<Form.FieldErrors />
					</Form.Field>

					{#if uploads.length > 0}
						<UploadList {uploads} oncancel={(id) => aborts.get(id)?.()} />
						<p class="text-xs text-muted-foreground">
							The issue is created. Leave this open until the files finish, or they will not reach
							it.
						</p>
					{:else if attaching.length > 0}
						<ul class="flex flex-col gap-1">
							{#each attaching as file (file.key)}
								<li
									class="flex items-center gap-2 rounded-md border border-line-subtle px-2 py-1.5 text-sm"
								>
									<Paperclip
										class="size-icon-row flex-none text-muted-foreground"
										aria-hidden="true"
									/>
									<span class="min-w-0 flex-1 truncate text-ink-900">{file.name}</span>
									<span class="flex-none text-xs tabular-nums text-muted-foreground">
										{formatBytes(file.size)}
									</span>
									<Button
										type="button"
										variant="ghost"
										size="icon-sm"
										aria-label="Remove {file.name}"
										disabled={busy}
										onclick={() => dropPending(file.key)}
									>
										<X aria-hidden="true" />
									</Button>
								</li>
							{/each}
						</ul>
						<p class="text-xs text-muted-foreground">
							{#if raised}
								{attaching.length === 1 ? "This file" : "These files"} did not upload. The issue is
								saved; Finish attaching tries {attaching.length === 1 ? "it" : "them"} again against
								it, and the issue's own fields can no longer be changed here.
							{:else}
								{attaching.length === 1 ? "This file is" : "These files are"} attached once the issue
								is created.
							{/if}
						</p>
					{/if}
				</div>

				<div class="flex flex-wrap gap-1.5 px-4 pb-3.5">
					<PropertyPicker
						options={stateOptions}
						placeholder="Set status…"
						empty={available.length === 0 ? "That team's states are still loading" : "No matches"}
						onpick={(value) => ($formData.stateId = value)}
					>
						{#snippet trigger(props)}
							<Button
								{...props}
								variant="outline"
								size="sm"
								disabled={busy || Boolean(raised)}
								class={chipClass}
							>
								{#if openState}
									<StatusIcon category={openState.category} decorative />
									{openState.name}
								{:else}
									<StatusIcon category="not_started" decorative />
									Status
								{/if}
							</Button>
						{/snippet}
						{#snippet mark(option)}
							{@const state = available.find((candidate) => candidate.id === option.value)}
							{#if state}
								<StatusIcon category={state.category} decorative />
							{/if}
						{/snippet}
					</PropertyPicker>

					<PropertyPicker
						options={priorityOptions}
						placeholder="Set priority…"
						class="w-49"
						onpick={(value) => ($formData.priority = value as typeof $formData.priority)}
					>
						{#snippet trigger(props)}
							<Button
								{...props}
								variant="outline"
								size="sm"
								disabled={busy || Boolean(raised)}
								class={chipClass}
							>
								<PriorityIcon priority={$formData.priority} />
								{$formData.priority === "none" ? "Priority" : priorityLabel($formData.priority)}
							</Button>
						{/snippet}
						{#snippet mark(option)}
							<PriorityIcon priority={option.value as typeof $formData.priority} />
						{/snippet}
					</PropertyPicker>

					<PropertyPicker
						options={assigneeOptions}
						placeholder="Assign to…"
						onpick={(value) => ($formData.assigneeId = value)}
					>
						{#snippet trigger(props)}
							<Button
								{...props}
								variant="outline"
								size="sm"
								disabled={busy || Boolean(raised)}
								class={chipClass}
							>
								{#if assignee}
									<Avatar.Root size="xs">
										<Avatar.Fallback>{initialsOf(assignee)}</Avatar.Fallback>
									</Avatar.Root>
									{assignee}
								{:else}
									<Avatar.Root size="xs" variant="ghost">
										<Avatar.Fallback>+</Avatar.Fallback>
									</Avatar.Root>
									Unassigned
								{/if}
							</Button>
						{/snippet}
						{#snippet mark(option)}
							{#if option.value}
								<Avatar.Root size="xs">
									<Avatar.Fallback>{initialsOf(option.label)}</Avatar.Fallback>
								</Avatar.Root>
							{:else}
								<Avatar.Root size="xs" variant="ghost">
									<Avatar.Fallback>+</Avatar.Fallback>
								</Avatar.Root>
							{/if}
						{/snippet}
					</PropertyPicker>

					<PropertyPicker
						options={labelOptions}
						placeholder="Add or create a label…"
						class="w-49"
						empty="No labels reach this team"
						closeOnPick={false}
						bind:search={labelSearch}
						onpick={toggleLabel}
					>
						{#snippet shortcut()}
							<Kbd keys="Esc" />
						{/snippet}
						{#snippet action(search)}
							{#if coinable}
								<Command.Item value="coin-{search}" onSelect={() => coinLabel(search)}>
									<span class="inline-flex w-3.75 flex-none justify-center">
										<Plus class="text-muted-foreground" aria-hidden="true" />
									</span>
									<span class="min-w-0 flex-1 truncate">
										{coining ? "Creating" : "Create"} “{search.trim()}”
									</span>
								</Command.Item>
							{:else if labelOptions.length === 0}
								<p class="px-2 py-1.5 text-sm text-muted-foreground">
									No labels reach this team. Type a name to create one.
								</p>
							{/if}
						{/snippet}
						{#snippet footer()}
							{#if labelFailure}
								<p class="border-t border-line-subtle px-2 py-1.5 text-sm text-destructive" role="alert">
									{labelFailure}
								</p>
							{/if}
						{/snippet}
						{#snippet trigger(props)}
							<Button
								{...props}
								variant="outline"
								size="sm"
								disabled={busy || Boolean(raised)}
								class={chipClass}
							>
								<Tags class="text-muted-foreground" aria-hidden="true" />
								{chosenLabels.length > 0
									? chosenLabels.map((label) => label.name).join(", ")
									: "Label"}
							</Button>
						{/snippet}
						{#snippet mark(option)}
							{@const label = labels.find((candidate) => candidate.id === option.value)}
							<LabelDot color={label?.color} />
						{/snippet}
					</PropertyPicker>

					<PropertyPicker
						options={projectOptions}
						placeholder="Move to project…"
						onpick={(value) => ($formData.projectId = value)}
					>
						{#snippet trigger(props)}
							<Button
								{...props}
								variant="outline"
								size="sm"
								disabled={busy || Boolean(raised)}
								class={chipClass}
							>
								{project ? project.name : "Project"}
							</Button>
						{/snippet}
					</PropertyPicker>

					<PropertyPicker
						options={dueOptions}
						placeholder="Set due date…"
						onpick={(value) => ($formData.dueOn = value)}
					>
						{#snippet trigger(props)}
							<Button
								{...props}
								variant="outline"
								size="sm"
								disabled={busy || Boolean(raised)}
								class={chipClass}
							>
								<CalendarDays class="text-muted-foreground" aria-hidden="true" />
								{$formData.dueOn ? onCalendarDate($formData.dueOn) : "Due date"}
							</Button>
						{/snippet}
						{#snippet mark()}
							<CalendarDays class="text-muted-foreground" aria-hidden="true" />
						{/snippet}
					</PropertyPicker>
				</div>

				{#if failure}
					<p class="px-4 pb-3 text-sm text-destructive" role="alert">{failure}</p>
				{/if}
			</div>

			<div
				class="flex flex-none flex-wrap items-center gap-x-3 gap-y-2 border-t border-line-subtle py-2.5 pr-3 pl-4"
			>
				<AttachmentPicker disabled={busy} onfiles={takeFiles} />
				<label class="flex min-w-0 items-center gap-2 text-md text-foreground">
					<Switch bind:checked={$formData.createMore} disabled={busy || Boolean(raised)} />
					Create more
				</label>
				<span class="flex-1"></span>
				<Button
					type="button"
					variant="ghost"
					size="sm"
					disabled={busy}
					onclick={() => (open = false)}
				>
					Cancel
				</Button>
				<Button
					type="submit"
					size="sm"
					class="ml-auto min-w-0"
					disabled={busy || unconfirmed || !$formData.title.trim()}
				>
					{#if uploads.length > 0}
						Attaching files
					{:else if busy}
						{raised ? "Finishing" : "Creating issue"}
					{:else if raised}
						Finish attaching
					{:else}
						Create issue
					{/if}
					<Kbd keys="⌘ ↵" tone="inverse" class="hidden sm:inline-flex" />
				</Button>
			</div>
		</form>
	</Dialog.Content>
</Dialog.Root>

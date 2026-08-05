<script lang="ts">
	import CalendarDays from "@lucide/svelte/icons/calendar-days";
	import ChevronDown from "@lucide/svelte/icons/chevron-down";
	import ChevronRight from "@lucide/svelte/icons/chevron-right";
	import Tags from "@lucide/svelte/icons/tags";
	import X from "@lucide/svelte/icons/x";
	import { defaults, setError, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import { api } from "$lib/api";
	import * as Avatar from "$lib/components/ui/avatar/index.js";
	import * as Dialog from "$lib/components/ui/dialog/index.js";
	import * as Form from "$lib/components/ui/form/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { Switch } from "$lib/components/ui/switch/index.js";
	import { Textarea } from "$lib/components/ui/textarea/index.js";
	import Kbd from "$lib/components/norn/kbd.svelte";
	import PriorityIcon from "$lib/components/norn/priority-icon.svelte";
	import StatusIcon from "$lib/components/norn/status-icon.svelte";
	import TeamKey from "$lib/components/norn/team-key.svelte";
	import LabelDot from "$lib/labels/label-dot.svelte";
	import { initialsOf } from "$lib/team/members";
	import { onCalendarDate } from "$lib/time";
	import type { Label } from "$lib/labels/labels";
	import type { Project } from "$lib/projects/projects";
	import type { WorkflowState } from "$lib/team/states";
	import type { Team } from "$lib/team/teams";
	import PropertyPicker, { type PickerOption } from "./property-picker.svelte";
	import { duePresets } from "./facets";
	import { newIssueSchema } from "./new-issue-schema";
	import { issueFailureMessage, priorities, priorityLabel, readIssueFailure } from "./issues";
	import type { Issue } from "./issues";

	let {
		open = $bindable(false),
		workspaceId,
		teams,
		states,
		members,
		labels,
		projects,
		today,
		prefill,
		oncreated,
	}: {
		open?: boolean;
		workspaceId: string;
		teams: Team[];
		states: Record<string, WorkflowState[]>;
		members: { accountId: string; displayName?: string }[];
		labels: Label[];
		projects: Project[];
		today: string;
		prefill?: Partial<{
			teamId: string;
			stateId: string;
			priority: string;
			assigneeId: string;
			projectId: string;
		}>;
		oncreated?: (issue: Issue) => void;
	} = $props();

	let failure = $state<string | null>(null);
	let teamStates = $state.raw<Record<string, WorkflowState[]>>({});
	let titleField = $state<HTMLInputElement | null>(null);

	const form = superForm(defaults(zod4(newIssueSchema)), {
		id: "new-issue",
		SPA: true,
		validators: zod4Client(newIssueSchema),
		resetForm: false,
		onUpdate: async ({ form: pending }) => {
			if (!pending.valid) return;

			failure = null;

			const { data: created, error } = await api.POST("/workspaces/{workspaceId}/issues", {
				params: { path: { workspaceId } },
				body: {
					teamId: pending.data.teamId,
					title: pending.data.title,
					description: pending.data.description || undefined,
					priority: pending.data.priority,
					stateId: pending.data.stateId || undefined,
					assigneeId: pending.data.assigneeId || undefined,
					projectId: pending.data.projectId || undefined,
					labelIds: pending.data.labelIds.length > 0 ? pending.data.labelIds : undefined,
					dueOn: pending.data.dueOn || undefined,
				},
			});

			if (error || !created) {
				const read = readIssueFailure(error);

				if (read.kind === "invalid") {
					for (const field of read.fields) {
						if (field === "title") setError(pending, "title", "Give the issue a title.");
						if (field === "dueOn") setError(pending, "dueOn", "Use a date like 2026-09-01.");
					}
				}

				failure = issueFailureMessage(read);

				return;
			}

			oncreated?.(created);

			if (!pending.data.createMore) {
				open = false;

				return;
			}

			formData.update((current) => ({ ...current, title: "", description: "" }), {
				taint: false,
			});
			titleField?.focus();
		},
	});

	const { form: formData, enhance, submitting } = form;

	const team = $derived(teams.find((candidate) => candidate.id === $formData.teamId) ?? teams[0]);

	const available = $derived(team ? (states[team.id] ?? teamStates[team.id] ?? []) : []);

	const openState = $derived(available.find((state) => state.id === $formData.stateId));

	const assignee = $derived(
		members.find((member) => member.accountId === $formData.assigneeId)?.displayName ?? ""
	);

	const project = $derived(projects.find((candidate) => candidate.id === $formData.projectId));

	const chosenLabels = $derived(labels.filter((label) => $formData.labelIds.includes(label.id)));

	const reachable = $derived(labels.filter((label) => !label.teamId || label.teamId === team?.id));

	$effect(() => {
		if (!open) return;

		form.reset({ keepMessage: false });
		failure = null;

		formData.update(
			(current) => ({
				...current,
				title: "",
				description: "",
				labelIds: [],
				dueOn: "",
				createMore: false,
				teamId: prefill?.teamId || teams[0]?.id || "",
				stateId: prefill?.stateId ?? "",
				priority: (prefill?.priority as typeof current.priority) ?? "none",
				assigneeId: prefill?.assigneeId ?? "",
				projectId: prefill?.projectId ?? "",
			}),
			{ taint: false }
		);
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
		void loadStates(teamId);
	}

	function submitOnMeta(event: KeyboardEvent & { currentTarget: HTMLElement }) {
		if (!(event.metaKey || event.ctrlKey) || event.key !== "Enter") return;

		event.preventDefault();
		event.currentTarget.closest("form")?.requestSubmit();
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
		...members.map((member) => ({
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

<Dialog.Root bind:open>
	<Dialog.Content class="top-21 p-0 sm:max-w-162" showCloseButton={false}>
		<Dialog.Description class="sr-only">
			Raise an issue in a team you can see, and set its properties before it is created.
		</Dialog.Description>

		<form method="POST" use:enhance>
			<div class="flex items-center gap-2 border-b border-line-subtle py-2.75 pr-2.5 pl-3.5">
				<PropertyPicker
					options={teamOptions}
					placeholder="Move to team…"
					onpick={chooseTeam}
					class="w-51.5"
				>
					{#snippet trigger(props)}
						<Button {...props} variant="ghost" size="sm" class="gap-1.5 px-1.5 font-medium">
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
					onclick={() => (open = false)}
				>
					<X aria-hidden="true" />
				</Button>
			</div>

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
								disabled={$submitting}
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
							<Textarea
								{...props}
								bind:value={$formData.description}
								variant="seamless"
								rows={3}
								disabled={$submitting}
								onkeydown={submitOnMeta}
								placeholder="Add description… What is broken, what should happen instead."
								class="p-0"
							/>
						{/snippet}
					</Form.Control>
					<Form.FieldErrors />
				</Form.Field>
			</div>

			<div class="flex flex-wrap gap-1.5 px-4 pb-3.5">
				<PropertyPicker
					options={stateOptions}
					placeholder="Set status…"
					empty={available.length === 0 ? "That team's states are still loading" : "No matches"}
					onpick={(value) => ($formData.stateId = value)}
				>
					{#snippet trigger(props)}
						<Button {...props} variant="outline" size="sm" class={chipClass}>
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
						<Button {...props} variant="outline" size="sm" class={chipClass}>
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
						<Button {...props} variant="outline" size="sm" class={chipClass}>
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
					placeholder="Add label…"
					class="w-49"
					empty="No labels reach this team"
					closeOnPick={false}
					onpick={toggleLabel}
				>
					{#snippet shortcut()}
						<Kbd keys="Esc" />
					{/snippet}
					{#snippet trigger(props)}
						<Button {...props} variant="outline" size="sm" class={chipClass}>
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
						<Button {...props} variant="outline" size="sm" class={chipClass}>
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
						<Button {...props} variant="outline" size="sm" class={chipClass}>
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

			<div class="flex items-center gap-3 border-t border-line-subtle py-2.5 pr-3 pl-4">
				<label class="flex items-center gap-2 text-md text-foreground">
					<Switch bind:checked={$formData.createMore} disabled={$submitting} />
					Create more
				</label>
				<span class="flex-1"></span>
				<Button
					type="button"
					variant="ghost"
					size="sm"
					disabled={$submitting}
					onclick={() => (open = false)}
				>
					Cancel
				</Button>
				<Button type="submit" size="sm" disabled={$submitting || !$formData.title.trim()}>
					{$submitting ? "Creating issue" : "Create issue"}
					<Kbd keys="⌘ ↵" tone="inverse" />
				</Button>
			</div>
		</form>
	</Dialog.Content>
</Dialog.Root>

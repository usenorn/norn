<script lang="ts" module>
	export type SourcePickerOpen = "none" | "project" | "issue";
</script>

<script lang="ts">
	import Plus from "@lucide/svelte/icons/plus";
	import { Button } from "$lib/components/ui/button/index.js";
	import PropertyPicker, { type PickerOption } from "$lib/issues/property-picker.svelte";
	import { findIssues, type Suggestion, type SuggestionOutcome } from "$lib/editor/search";
	import type { Project } from "$lib/projects/projects";
	import { issueSearchDelayMs, type EntrySource, type RoadmapEntryKind } from "./admin";

	type IssueSearch = SuggestionOutcome | { state: "searching" };

	let {
		kinds,
		projects,
		selected,
		workspaceId,
		workspace,
		disabled,
		opened = "none",
		outcome,
		onpick,
	}: {
		kinds: RoadmapEntryKind[];
		projects: Project[];
		selected: Set<string>;
		workspaceId: string;
		workspace: string;
		disabled: boolean;
		opened?: SourcePickerOpen;
		outcome?: SuggestionOutcome;
		onpick: (source: EntrySource) => void;
	} = $props();

	// svelte-ignore state_referenced_locally
	let projectOpen = $state(opened === "project");
	// svelte-ignore state_referenced_locally
	let issueOpen = $state(opened === "issue");
	let query = $state("");
	let found = $state.raw<IssueSearch>({ state: "typing" });

	const search = $derived<IssueSearch>(outcome ?? found);

	const projectOptions = $derived<PickerOption[]>(
		projects.map((project) => ({
			value: project.id,
			label: project.name,
			disabled: selected.has(project.id),
			hint: selected.has(project.id) ? "Added" : undefined,
		}))
	);

	const issues = $derived<Suggestion[]>(
		search.state === "ready" ? search.groups.flatMap((group) => group.items.filter((item) => item.kind === "issue")) : []
	);

	const issueOptions = $derived<PickerOption[]>(
		issues.map((issue) => ({
			value: issue.id,
			label: issue.label,
			disabled: selected.has(issue.id),
			hint: selected.has(issue.id) ? "Added" : issue.hint,
		}))
	);

	const issueEmpty = $derived.by(() => {
		switch (search.state) {
			case "typing":
				return "Type a title, or a key like ENG-7";
			case "searching":
				return "Searching…";
			case "failed":
				return "Search is not answering. Try again in a moment.";
			default:
				return "No issue you can see matches";
		}
	});

	const offered = new Map<string, Suggestion>();

	let pending: { timer: ReturnType<typeof setTimeout>; controller: AbortController } | undefined;

	function ask(value: string) {
		query = value;

		if (pending) {
			clearTimeout(pending.timer);
			pending.controller.abort();
			pending = undefined;
		}

		const asked = value.trim();

		if (asked === "") {
			found = { state: "typing" };

			return;
		}

		found = { state: "searching" };

		const controller = new AbortController();
		const timer = setTimeout(async () => {
			const answer = await findIssues(workspaceId, workspace, asked, controller.signal).catch(
				(): SuggestionOutcome => ({ state: "failed" })
			);

			if (controller.signal.aborted) return;

			if (answer.state === "ready") {
				for (const item of answer.groups.flatMap((group) => group.items)) offered.set(item.id, item);
			}

			found = answer;
		}, issueSearchDelayMs);

		pending = { timer, controller };
	}

	function pickProject(id: string) {
		const project = projects.find((candidate) => candidate.id === id);

		if (project) onpick({ id: project.id, kind: "project", label: project.name });
	}

	function pickIssue(id: string) {
		const issue = offered.get(id) ?? issues.find((candidate) => candidate.id === id);

		if (issue?.kind === "issue") onpick({ id: issue.id, kind: "issue", label: issue.label, reference: issue.reference });
	}
</script>

<div class="flex flex-wrap items-center gap-2">
	{#if kinds.includes("project")}
		<PropertyPicker
			options={projectOptions}
			placeholder="Find a project"
			empty={projects.length === 0 ? "This workspace has no projects yet" : "No project matches"}
			bind:open={projectOpen}
			onpick={pickProject}
			align="end"
			class="w-72"
		>
			{#snippet trigger(props)}
				<Button {...props} size="sm" variant="secondary" {disabled}>
					<Plus aria-hidden="true" />
					Add project
				</Button>
			{/snippet}
		</PropertyPicker>
	{/if}

	{#if kinds.includes("issue")}
		<PropertyPicker
			options={issueOptions}
			placeholder="Search issues"
			empty={issueEmpty}
			filter={false}
			bind:open={issueOpen}
			bind:search={() => query, ask}
			onpick={pickIssue}
			align="end"
			class="w-72"
		>
			{#snippet trigger(props)}
				<Button {...props} size="sm" variant="secondary" {disabled}>
					<Plus aria-hidden="true" />
					Add issue
				</Button>
			{/snippet}
		</PropertyPicker>
	{/if}
</div>

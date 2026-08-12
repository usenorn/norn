import type { Label } from "$lib/labels/labels";
import type { Project } from "$lib/projects/projects";
import type { WorkflowState } from "$lib/team/states";
import type { Team } from "$lib/team/teams";
import type { Issue } from "./issues";
import type { NewIssueInput } from "./new-issue-schema";

export type IssueCreation = { key: string; issue: Issue; settled: boolean };

export type CreationOutcome =
	| { key: string; kind: "created"; issue: Issue }
	| { key: string; kind: "refused"; failure: string; input?: NewIssueInput };

export type DraftContext = {
	workspaceId: string;
	team: Team;
	state: WorkflowState;
	labels: Label[];
	projects: Project[];
	now: string;
};

export function draftIssue(key: string, input: NewIssueInput, context: DraftContext): Issue {
	return {
		id: key,
		workspaceId: context.workspaceId,
		teamId: context.team.id,
		teamKey: context.team.key,
		referenceKey: context.team.key,
		number: 0,
		reference: context.team.key,
		version: 0,
		title: input.title.trim(),
		description: input.description,
		priority: input.priority,
		assigneeAccountId: input.assigneeId || undefined,
		dueOn: input.dueOn || undefined,
		projectId: input.projectId || undefined,
		projectName: context.projects.find((project) => project.id === input.projectId)?.name,
		labels: context.labels.filter((label) => input.labelIds.includes(label.id)),
		state: {
			id: context.state.id,
			name: context.state.name,
			category: context.state.category,
			position: context.state.position,
		},
		status: "active",
		stateEnteredAt: context.now,
		createdAt: context.now,
	};
}

export function withDraft(held: IssueCreation[], key: string, issue: Issue): IssueCreation[] {
	return [{ key, issue, settled: false }, ...held];
}

export function settledWith(held: IssueCreation[], outcome: CreationOutcome): IssueCreation[] {
	if (outcome.kind === "refused") {
		return held.filter((creation) => creation.key !== outcome.key);
	}

	return held.map((creation) =>
		creation.key === outcome.key ? { key: creation.key, issue: outcome.issue, settled: true } : creation
	);
}

export function unsettled(held: IssueCreation[]): IssueCreation[] {
	return held.filter((creation) => !creation.settled);
}

import type { Issue, IssueProgress } from "#lib/issues/board.js";
import type { Display, IssueLayout, IssueTab } from "#lib/issues/display.js";
import type { Facets } from "#lib/issues/facets.js";
import type { IssueGroupTally, IssueQueryBody } from "#lib/issues/filter.js";
import type { BulkActionResult } from "#lib/issues/bulk.js";
import type { ColumnPage } from "#lib/issues/paging.js";
import type { Label } from "#lib/labels/labels.js";
import type { Membership } from "#lib/workspace/members.js";
import type { Project } from "#lib/projects/projects.js";
import type { TeamCycle } from "#lib/cycles/cycles.js";
import type { Team } from "#lib/team/teams.js";
import type { WorkflowState } from "#lib/team/states.js";
import type { AppliedView } from "#lib/views/applied.js";

export type IssuesListingData = {
	team: Team | null;
	teams: Team[];
	applied: AppliedView;
	query: IssueQueryBody;
	issues: Issue[] | undefined;
	nextCursor: string | undefined;
	groups: IssueGroupTally[] | undefined;
	totals: IssueGroupTally[] | undefined;
	states: WorkflowState[];
	progress: IssueProgress | undefined;
	facets: Facets;
	display: Display;
	today: string;
	tab: IssueTab;
	layout: IssueLayout;
};

export type IssuesListingScope = {
	now: string;
	workspace: { id: string; slug: string; name: string; timezone: string };
	members: Membership[];
	labels: Label[];
	projects: Project[];
	cycles: TeamCycle[];
};

export type IssuesPreview = {
	team: Team | null;
	teams?: Team[];
	applied?: AppliedView;
	bulk?: BulkActionResult;
	states?: WorkflowState[];
	issues?: Issue[];
	labels?: Label[];
	nextCursor?: string;
	groups?: IssueGroupTally[];
	totals?: IssueGroupTally[];
	pages?: Record<string, ColumnPage>;
	progress?: IssueProgress;
	members?: Membership[];
	facets?: Facets;
	display?: Display;
};

export function issuesPath(workspace: string): string {
	return `/${workspace}/issues`;
}

export function teamIssuesPath(workspace: string, teamKey: string): string {
	return `/${workspace}/teams/${teamKey.toUpperCase()}/issues`;
}

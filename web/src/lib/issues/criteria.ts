import { api } from "$lib/api";
import type { components } from "$lib/api/dashboard.gen";

export type AcceptanceCriterion = components["schemas"]["AcceptanceCriterion"];
export type CriterionEvidence = components["schemas"]["CriterionEvidence"];
export type EvidenceKind = components["schemas"]["EvidenceKind"];

export type CriteriaPanel =
	| { kind: "loading" }
	| { kind: "empty" }
	| { kind: "ready"; criteria: AcceptanceCriterion[] }
	| { kind: "unavailable" };

export const evidenceKinds: { kind: EvidenceKind; label: string; hint: string }[] = [
	{ kind: "test", label: "A test", hint: "Where the run can be read" },
	{ kind: "screenshot", label: "A screenshot", hint: "Where the picture is" },
	{ kind: "pull_request", label: "A pull request", hint: "Where the change is" },
	{ kind: "person", label: "Somebody checked it", hint: "Who checked, and what they did" },
];

export const evidenceLabels: Record<EvidenceKind, string> = {
	test: "Test",
	screenshot: "Screenshot",
	pull_request: "Pull request",
	person: "Checked by hand",
};

export function needsAddress(kind: EvidenceKind): boolean {
	return kind !== "person";
}

export function provenLine(criteria: AcceptanceCriterion[]): string {
	const proven = criteria.filter((criterion) => criterion.proven).length;

	if (criteria.length === 0) return "";

	return `${proven} of ${criteria.length} proven`;
}

export async function readCriteria(
	workspaceId: string,
	issueId: string
): Promise<AcceptanceCriterion[] | undefined> {
	try {
		const { data, error } = await api.GET(
			"/workspaces/{workspaceId}/issues/{issueId}/criteria",
			{ params: { path: { workspaceId, issueId } } }
		);

		return error || !data ? undefined : data.criteria;
	} catch {
		return undefined;
	}
}

export async function fileEvidence(
	workspaceId: string,
	issueId: string,
	body: { criterionId: string; kind: EvidenceKind; label: string; url?: string }
): Promise<boolean> {
	try {
		const { error } = await api.POST("/workspaces/{workspaceId}/issues/{issueId}/criteria", {
			params: { path: { workspaceId, issueId } },
			body,
		});

		return !error;
	} catch {
		return false;
	}
}

export async function dropEvidence(
	workspaceId: string,
	issueId: string,
	evidenceId: string
): Promise<boolean> {
	try {
		const { error } = await api.DELETE(
			"/workspaces/{workspaceId}/issues/{issueId}/criteria/evidence/{evidenceId}",
			{ params: { path: { workspaceId, issueId, evidenceId } } }
		);

		return !error;
	} catch {
		return false;
	}
}

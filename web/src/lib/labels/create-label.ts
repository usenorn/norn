import { api } from "#lib/api/index.js";
import { attempt } from "#lib/api/attempt.js";
import type { MembershipRole } from "#lib/workspace/members.js";
import { randomLabelColor, refusedLabelFailure, type Label, type LabelFailure } from "./labels";

export type CreatedLabel = { kind: "created"; label: Label } | { kind: "failed"; failure: LabelFailure };

export async function createLabel(workspaceId: string, name: string): Promise<CreatedLabel> {
	const outcome = await attempt({
		run: () =>
			api.POST("/workspaces/{workspaceId}/labels", {
				params: { path: { workspaceId } },
				body: { name: name.trim(), color: randomLabelColor() },
			}),
	});

	if (outcome.kind === "done") return { kind: "created", label: outcome.value };
	if (outcome.kind === "refused") {
		return { kind: "failed", failure: refusedLabelFailure(outcome.problem, outcome.status) };
	}

	return { kind: "failed", failure: { kind: "unavailable" } };
}

export function canCreateLabels(role: MembershipRole | undefined): boolean {
	return role === "admin";
}

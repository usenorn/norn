import type { components } from "$lib/api/dashboard.gen";

export type AuditEvent = components["schemas"]["AuditEvent"];
export type AuditAction = components["schemas"]["AuditAction"];
export type AuditActor = components["schemas"]["AuditActor"];
export type AuditAvailability = components["schemas"]["AuditAvailability"];

export type AuditRecord =
	| { kind: "loading" }
	| { kind: "empty"; retentionDays: number }
	| { kind: "ready"; events: AuditEvent[]; nextCursor?: string; retentionDays: number }
	| { kind: "unlicensed"; retentionDays: number }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

export type AuditFilters = {
	action: AuditAction | "";
	actor: string;
	resourceKind: string;
	from: string;
	to: string;
};

export const noAuditFilters: AuditFilters = {
	action: "",
	actor: "",
	resourceKind: "",
	from: "",
	to: "",
};

export function filtersApplied(filters: AuditFilters): boolean {
	return Object.values(filters).some((value) => value !== "");
}

export const auditActions: AuditAction[] = [
	"session.signed_in",
	"session.sign_in_failed",
	"session.signed_out",
	"session.revoked",
	"account.password_changed",
	"account.password_reset",
	"account.email_changed",
	"account.deactivated",
	"account.deleted",
	"membership.added",
	"membership.role_changed",
	"membership.removed",
	"membership.audit_access_changed",
	"team_membership.added",
	"team_membership.removed",
	"invitation.created",
	"invitation.revoked",
	"invitation.accepted",
	"sso.connection_saved",
	"sso.connection_removed",
	"sso.enforcement_changed",
	"sso.recovery_codes_issued",
	"sso.recovery_code_redeemed",
	"sso.identity_unlinked",
	"sso.identity_linked",
	"sso.identity_refused",
	"sso.account_opened",
	"token.minted",
	"token.revoked",
	"agent.registered",
	"agent.disabled",
	"agent.proposal_decided",
	"runner.enrolled",
	"runner.revoked",
	"codebase.connected",
	"codebase.disconnected",
	"webhook.registered",
	"webhook.removed",
	"webhook.disabled",
	"workspace.updated",
	"workspace.deletion_requested",
	"workspace.restored",
	"workspace.purged",
	"directory.connected",
	"directory.disconnected",
	"directory.token_rotated",
	"audit.exported",
	"access.denied",
];

const actionLabels: Record<AuditAction, string> = {
	"session.signed_in": "Signed in",
	"session.sign_in_failed": "Sign-in failed",
	"session.signed_out": "Signed out",
	"session.revoked": "Session revoked",
	"account.password_changed": "Password changed",
	"account.password_reset": "Password reset",
	"account.email_changed": "Email changed",
	"account.deactivated": "Account deactivated",
	"account.deleted": "Account deleted",
	"membership.added": "Member added",
	"membership.role_changed": "Role changed",
	"membership.removed": "Member removed",
	"membership.audit_access_changed": "Audit access changed",
	"team_membership.added": "Added to team",
	"team_membership.removed": "Removed from team",
	"invitation.created": "Invitation sent",
	"invitation.revoked": "Invitation revoked",
	"invitation.accepted": "Invitation accepted",
	"sso.connection_saved": "Single sign-on saved",
	"sso.connection_removed": "Single sign-on removed",
	"sso.enforcement_changed": "Sign-in requirement changed",
	"sso.recovery_codes_issued": "Recovery codes issued",
	"sso.recovery_code_redeemed": "Recovery code redeemed",
	"sso.identity_unlinked": "Single sign-on unlinked",
	"sso.identity_linked": "Single sign-on connected",
	"sso.identity_refused": "Single sign-on refused",
	"sso.account_opened": "Account opened by single sign-on",
	"token.minted": "Token created",
	"token.revoked": "Token revoked",
	"agent.registered": "Agent registered",
	"agent.disabled": "Agent disabled",
	"runner.enrolled": "Runner connected",
	"runner.revoked": "Runner revoked",
	"codebase.connected": "Codebase connected",
	"codebase.disconnected": "Codebase disconnected",
	"agent.proposal_decided": "Agent proposal decided",
	"webhook.registered": "Webhook registered",
	"webhook.removed": "Webhook removed",
	"webhook.disabled": "Webhook disabled",
	"workspace.updated": "Workspace updated",
	"workspace.deletion_requested": "Deletion requested",
	"workspace.restored": "Workspace restored",
	"workspace.purged": "Workspace purged",
	"directory.connected": "Directory connected",
	"directory.disconnected": "Directory disconnected",
	"directory.token_rotated": "Directory credential replaced",
	"audit.exported": "Audit log exported",
	"access.denied": "Access denied",
};

export function actionLabel(action: AuditAction): string {
	return actionLabels[action] ?? action;
}

const authLabels: Record<string, string> = {
	password: "Password",
	sso: "Single sign-on",
	api_token: "API token",
	agent: "Agent",
	system: "Norn",
};

export function actorLabel(actor: AuditActor): string {
	if (actor.kind === "agent") return actor.agentName || actor.name || "An agent";
	if (actor.kind === "token") return actor.tokenName ? `${actor.name ?? "A token"} · ${actor.tokenName}` : actor.name || "An API token";
	if (actor.kind === "system") return "Norn";

	return actor.name || "A deleted account";
}

export function authLabel(actor: AuditActor): string {
	return authLabels[actor.authMethod] ?? actor.authMethod;
}

export function resourceLabel(event: AuditEvent): string {
	if (event.resourceName) return event.resourceName;
	if (event.resourceKind) return event.resourceKind.replace(/_/g, " ");

	return "—";
}

export function detailPairs(event: AuditEvent): [string, string][] {
	return Object.entries(event.detail ?? {}).map(([key, value]) => [key.replace(/_/g, " "), value]);
}

export const resourceKinds = [
	"workspace",
	"membership",
	"team_membership",
	"invitation",
	"account",
	"session",
	"api_token",
	"agent",
	"sso_connection",
	"auth_policy",
	"audit_log",
];

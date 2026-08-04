import type { components, operations } from "$lib/api/dashboard.gen";
import type { MembershipRole } from "./members";

export type InvitationWorkspace = components["schemas"]["InvitationWorkspace"];

export type AcceptInvitation =
	| { kind: "no_token" }
	| { kind: "invalid" }
	| { kind: "expired" }
	| { kind: "revoked" }
	| { kind: "already_accepted" }
	| { kind: "create_account"; workspace: InvitationWorkspace; email: string; role: MembershipRole }
	| { kind: "sign_in_required"; workspace: InvitationWorkspace; email: string }
	| { kind: "confirm"; workspace: InvitationWorkspace; email: string; role: MembershipRole }
	| { kind: "address_mismatch"; email: string }
	| { kind: "sso_required"; workspace: InvitationWorkspace; email: string }
	| { kind: "joined"; workspace: InvitationWorkspace }
	| { kind: "unavailable" };

type PreviewResponses = operations["previewInvitation"]["responses"];
type AcceptResponses = operations["acceptInvitation"]["responses"];

type CodedPreviewProblem = PreviewResponses[400 | 409]["content"]["application/problem+json"];
type CodedAcceptProblem = AcceptResponses[400 | 409]["content"]["application/problem+json"];

export type PreviewProblem =
	PreviewResponses[400 | 404 | 409 | 500]["content"]["application/problem+json"];

export type AcceptProblem =
	AcceptResponses[400 | 403 | 404 | 409 | 422 | 429 | 500 | 503]["content"]["application/problem+json"];

export type InvitationPreview = PreviewResponses[200]["content"]["application/json"];

function coded(problem: PreviewProblem | AcceptProblem): problem is CodedPreviewProblem | CodedAcceptProblem {
	return "code" in problem;
}

export type InvitationContext = { workspace: InvitationWorkspace; email: string };

export function linkFailure(
	problem: PreviewProblem | AcceptProblem,
	context?: InvitationContext
): AcceptInvitation {
	if (!coded(problem)) return { kind: "unavailable" };

	switch (problem.code) {
		case "invitation_expired":
			return { kind: "expired" };
		case "invitation_invalid":
			return { kind: "invalid" };
		case "invitation_revoked":
			return { kind: "revoked" };
		case "invitation_accepted":
			return { kind: "already_accepted" };
		case "invitation_address_mismatch":
			return { kind: "address_mismatch", email: problem.invitedEmail ?? context?.email ?? "" };
		case "account_exists":
			return context
				? { kind: "sign_in_required", ...context }
				: { kind: "already_accepted" };
		default:
			return { kind: "unavailable" };
	}
}

export function invitationState(
	preview: InvitationPreview,
	signedInAs: string | null
): AcceptInvitation {
	const { workspace, email, role } = preview;

	if (preview.ssoEnforced) return { kind: "sso_required", workspace, email };

	if (signedInAs) {
		return signedInAs.toLowerCase() === email.toLowerCase()
			? { kind: "confirm", workspace, email, role }
			: { kind: "address_mismatch", email };
	}

	return preview.accountExists
		? { kind: "sign_in_required", workspace, email }
		: { kind: "create_account", workspace, email, role };
}

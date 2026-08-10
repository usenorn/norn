import type { components, operations } from "$lib/api/dashboard.gen";
import type { MembershipRole } from "./members";
import type { SignedInAccount } from "$lib/account/accounts";

export type InvitationWorkspace = components["schemas"]["InvitationWorkspace"];

export type InvitationInviter = components["schemas"]["InvitationInviter"];

export type InvitationDetail = {
	workspace: InvitationWorkspace;
	email: string;
	invitedBy?: InvitationInviter;
	invitedAt: string;
	expiresAt: string;
	teams: string[];
};

export type AcceptInvitation =
	| { kind: "no_token" }
	| { kind: "invalid" }
	| { kind: "expired" }
	| { kind: "revoked" }
	| { kind: "already_accepted" }
	| ({ kind: "create_account"; role: MembershipRole } & InvitationDetail)
	| ({ kind: "sign_in_required" } & InvitationDetail)
	| ({ kind: "confirm"; role: MembershipRole; slot: string } & InvitationDetail)
	| { kind: "address_mismatch"; email: string; signedIn: SignedInAccount[] }
	| ({ kind: "sso_required" } & InvitationDetail)
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

export type InvitationContext = InvitationDetail;

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
			return {
				kind: "address_mismatch",
				email: problem.invitedEmail ?? context?.email ?? "",
				signedIn: [],
			};
		case "account_exists":
			return context ? { kind: "sign_in_required", ...context } : { kind: "already_accepted" };
		default:
			return { kind: "unavailable" };
	}
}

export function invitationState(
	preview: InvitationPreview,
	signedIn: SignedInAccount[]
): AcceptInvitation {
	const { workspace, email, role } = preview;

	const detail: InvitationDetail = {
		workspace,
		email,
		invitedBy: preview.invitedBy,
		invitedAt: preview.invitedAt,
		expiresAt: preview.expiresAt,
		teams: preview.teams,
	};

	if (preview.ssoEnforced) return { kind: "sso_required", ...detail };

	// An invitation names one address, so among several signed-in accounts only the one holding it
	// may accept; the rest are offered as a reason to sign in as the invitee instead.
	const invitee = signedIn.find(
		(candidate) => candidate.account.email.toLowerCase() === email.toLowerCase()
	);

	if (invitee) return { kind: "confirm", role, slot: invitee.defaultSlot, ...detail };

	if (signedIn.length > 0) return { kind: "address_mismatch", email, signedIn };

	return preview.accountExists
		? { kind: "sign_in_required", ...detail }
		: { kind: "create_account", role, ...detail };
}

export function invitedHeadline(detail: InvitationDetail): string {
	return detail.invitedBy
		? `${detail.invitedBy.name} invited you to ${detail.workspace.name}`
		: `You were invited to ${detail.workspace.name}`;
}

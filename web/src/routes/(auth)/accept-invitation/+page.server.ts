import { fail } from "@sveltejs/kit";
import { message, setError, superValidate, type Infer } from "sveltekit-superforms";
import { zod4 } from "sveltekit-superforms/adapters";
import { passwordMessage } from "$lib/auth/password-reset";
import { acceptInvitationSchema } from "$lib/workspace/accept-invitation-schema";
import {
	invitationState,
	linkFailure,
	type AcceptInvitation,
	type InvitationContext,
} from "$lib/workspace/accept-invitation";
import { acceptInvitationPreviewStates } from "./preview";
import type { Actions, PageServerLoad } from "./$types";

type AcceptInvitationForm = Infer<typeof acceptInvitationSchema>;

const formId = "accept-invitation-form";
const fallbackTimezone = "UTC";

type AcceptInvitationData = {
	token: string | null;
	invitation: AcceptInvitation;
	signedInAs: string;
};

export const load: PageServerLoad = async ({ locals, url }) => ({
	...(await invitationFor(locals, url, url.searchParams.get("token"))),
	form: await superValidate<AcceptInvitationForm, AcceptInvitation>(zod4(acceptInvitationSchema), {
		id: formId,
	}),
});

async function invitationFor(
	locals: App.Locals,
	url: URL,
	token: string | null
): Promise<AcceptInvitationData> {
	if (import.meta.env.DEV && acceptInvitationPreviewStates[url.searchParams.get("state") ?? ""]) {
		return { token, invitation: { kind: "no_token" }, signedInAs: "" };
	}

	if (!token) return { token: null, invitation: { kind: "no_token" }, signedInAs: "" };

	const [preview, signedIn] = await Promise.all([
		locals.api.POST("/invitations/preview", { body: { token } }),
		locals.signedIn,
	]);

	const signedInAs = signedIn.map((account) => account.account.email).join(", ");

	if (preview.error) return { token, invitation: linkFailure(preview.error), signedInAs };
	if (!preview.data) return { token, invitation: { kind: "unavailable" }, signedInAs };

	return {
		token,
		invitation: invitationState(preview.data, signedIn),
		signedInAs,
	};
}

async function accept(
	locals: App.Locals,
	url: URL,
	body: FormData,
	credentials: { displayName?: string; password?: string }
) {
	const token = String(body.get("token") ?? "");

	const form = await superValidate<AcceptInvitationForm, AcceptInvitation>(
		body,
		zod4(acceptInvitationSchema),
		{ id: formId }
	);

	clearSecrets(form);

	if (!token) return message(form, { kind: "invalid" }, { status: 400 });

	const { data, error } = await locals.api.POST("/invitations/accept", {
		body: {
			token,
			...credentials,
			timezone: String(body.get("timezone") || fallbackTimezone),
		},
	});

	if (data) {
		return message(form, {
			kind: "joined",
			workspace: { slug: data.workspace.slug, name: data.workspace.name },
		});
	}

	if (!error) return message(form, { kind: "unavailable" }, { status: 500 });

	const { invitation } = await invitationFor(locals, url, token);
	const context = contextOf(invitation);
	const failure = linkFailure(error, context);

	if (failure.kind !== "unavailable" || !error.errors?.length) {
		return message(form, failure, { status: 400 });
	}

	for (const field of error.errors) {
		if (field.field === "password") setError(form, "password", passwordMessage(field.code));
		if (field.field === "display_name") setError(form, "name", "Enter your name.");
	}

	return fail(400, { form });
}

function clearSecrets(form: { data: AcceptInvitationForm }) {
	form.data.password = "";
	form.data.repeat = "";
}

function contextOf(invitation: AcceptInvitation): InvitationContext | undefined {
	return invitation.kind === "create_account" ||
		invitation.kind === "confirm" ||
		invitation.kind === "sign_in_required" ||
		invitation.kind === "sso_required"
		? invitation
		: undefined;
}

export const actions: Actions = {
	join: async ({ locals, request, url }) => {
		const body = await request.formData();
		const form = await superValidate<AcceptInvitationForm, AcceptInvitation>(
			body,
			zod4(acceptInvitationSchema),
			{ id: formId }
		);

		if (!form.valid) {
			clearSecrets(form);

			return fail(400, { form });
		}

		return accept(locals, url, body, {
			displayName: form.data.name,
			password: form.data.password,
		});
	},

	confirm: async ({ locals, request, url }) => accept(locals, url, await request.formData(), {}),
};

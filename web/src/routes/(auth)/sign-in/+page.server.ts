import { fail, redirect } from "@sveltejs/kit";
import { message, setError, superValidate, type Infer } from "sveltekit-superforms";
import { zod4 } from "sveltekit-superforms/adapters";
import { identifySchema, signInSchema } from "#lib/auth/sign-in-schema.js";
import { signInFailure } from "#lib/auth/sign-in.js";
import { codeEntry } from "#lib/auth/sign-in-code.js";
import { carriedURL, safeReturn } from "#lib/auth/return-to.js";
import { addingAccount, leaveIfSignedIn } from "#lib/auth/signed-in.js";
import {
	discoverSignIn,
	beginSso,
	soleProvider,
	stepFor,
	type SignInOutcome,
	type SignInStep,
} from "#lib/auth/sign-in-step.js";
import { reachWorkspaceSignIn, ssoEntryPoint, type WorkspaceEntry } from "#lib/auth/workspace-sign-in.js";
import type { Actions, PageServerLoad } from "./$types";

type IdentifyForm = Infer<typeof identifySchema>;
type PasswordForm = Infer<typeof signInSchema>;

const identifyFormId = "sign-in-identify";
const passwordFormId = "sign-in-password";

async function stepAgain(api: App.Locals["api"], email: string, workspace: string): Promise<SignInStep> {
	if (workspace) {
		const entry = await reachWorkspaceSignIn(api, workspace);

		return entry.kind === "ready" ? stepFor(entry.signIn, "", entry.signIn.workspace) : { kind: "identify" };
	}

	const found = await discoverSignIn(api, email);

	return found.kind === "found"
		? stepFor(found.options, email)
		: { kind: "methods", email, workspace: "", options: [{ method: "password" }] };
}

export const load: PageServerLoad = async ({ locals, url }) => {
	await leaveIfSignedIn(locals, url);

	const entry: WorkspaceEntry = await reachWorkspaceSignIn(locals.api, url.searchParams.get("workspace") ?? "");

	return {
		adding: addingAccount(url) && (await locals.signedIn).length > 0,
		cancelTo: safeReturn(url.searchParams.get("return")),
		entry,
		step: (entry.kind === "ready"
			? stepFor(entry.signIn, "", entry.signIn.workspace)
			: { kind: "identify" }) as SignInStep,
		identifyForm: await superValidate<IdentifyForm, SignInOutcome>(zod4(identifySchema), {
			id: identifyFormId,
		}),
		passwordForm: await superValidate<PasswordForm, SignInOutcome>(zod4(signInSchema), {
			id: passwordFormId,
		}),
	};
};

export const actions: Actions = {
	identify: async ({ locals, request, url }) => {
		const body = await request.formData();
		const form = await superValidate<IdentifyForm, SignInOutcome>(body, zod4(identifySchema), {
			id: identifyFormId,
		});

		if (!form.valid) return fail(400, { form });

		const email = form.data.email;
		const found = await discoverSignIn(locals.api, email);

		if (found.kind === "invalid") return setError(form, "email", "Enter a valid email address.");

		if (found.kind === "failed") {
			return message(form, { step: { kind: "identify" }, failure: found.failure }, { status: 400 });
		}

		const provider = soleProvider(found.options);

		if (provider) {
			const returnTo = safeReturn(carriedURL(url, body).searchParams.get("return"));
			const { data } = await beginSso(locals.api, provider, returnTo, email);

			redirect(303, data ? data.authorizationUrl : ssoEntryPoint(provider.workspace, returnTo));
		}

		const outcome: SignInOutcome = { step: stepFor(found.options, email) };
		const passwordForm = await superValidate<PasswordForm, SignInOutcome>(
			{ email },
			zod4(signInSchema),
			{ id: passwordFormId, errors: false }
		);

		form.message = outcome;
		passwordForm.message = outcome;

		return { form, passwordForm };
	},

	password: async ({ locals, request, url }) => {
		const body = await request.formData();
		const form = await superValidate<PasswordForm, SignInOutcome>(body, zod4(signInSchema), {
			id: passwordFormId,
		});

		const password = form.data.password;
		const workspace = String(body.get("workspace") ?? "");

		form.data.password = "";

		if (!form.valid) {
			form.message = { step: await stepAgain(locals.api, form.data.email, workspace) };

			return fail(400, { form });
		}

		const { data, error } = await locals.api.POST("/auth/login", {
			body: { email: form.data.email, password },
		});

		if (data) redirect(303, codeEntry(data.challengeId, carriedURL(url, body)));

		return message(
			form,
			{
				step: await stepAgain(locals.api, form.data.email, workspace),
				failure: (error && signInFailure(error)) ?? { kind: "unavailable" },
			},
			{ status: 401 }
		);
	},
};

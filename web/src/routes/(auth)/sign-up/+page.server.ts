import { fail } from "@sveltejs/kit";
import { message, setError, superValidate, type Infer } from "sveltekit-superforms";
import { zod4 } from "sveltekit-superforms/adapters";
import { emailMessage, passwordMessage } from "$lib/auth/password-reset";
import { displayNameMessage, signUpFailure, signUpSent } from "$lib/auth/sign-up";
import { signUpSchema } from "$lib/auth/sign-up-schema";
import type { SignUpOutcome, SignUpResend } from "$lib/auth/types";
import type { Actions, PageServerLoad } from "./$types";

type SignUpForm = Infer<typeof signUpSchema>;

export type SignUpMessage = { outcome: SignUpOutcome | null; resend: SignUpResend };

const fallbackTimezone = "UTC";

export const load: PageServerLoad = async () => ({
	form: await superValidate<SignUpForm, SignUpMessage>(zod4(signUpSchema)),
});

async function requestSignUp(api: App.Locals["api"], body: FormData, resend: boolean) {
	const form = await superValidate<SignUpForm, SignUpMessage>(body, zod4(signUpSchema));

	const password = form.data.password;

	form.data.password = "";
	form.data.passwordConfirm = "";

	if (!form.valid) return fail(400, { form });

	const { data, error } = await api.POST("/auth/sign-up", {
		body: {
			email: form.data.email,
			displayName: form.data.name,
			timezone: String(body.get("timezone") || fallbackTimezone),
			password,
		},
	});

	if (data) return message(form, { outcome: signUpSent(data), resend: "idle" });
	if (!error) return message(form, { outcome: { kind: "unavailable" }, resend: "idle" });

	const failure = signUpFailure(error);

	if (failure) {
		return resend && failure.kind === "rate_limited"
			? message(form, { outcome: null, resend: "limited" }, { status: 429 })
			: message(form, { outcome: failure, resend: "idle" }, { status: 400 });
	}

	let handled = false;

	for (const field of error.errors ?? []) {
		if (field.field === "email") {
			setError(form, "email", emailMessage(field.code));
			handled = true;
		}
		if (field.field === "display_name") {
			setError(form, "name", displayNameMessage(field.code));
			handled = true;
		}
		if (field.field === "password") {
			setError(form, "password", passwordMessage(field.code));
			handled = true;
		}
	}

	if (handled) return fail(400, { form });

	return resend
		? message(form, { outcome: null, resend: "unavailable" }, { status: 500 })
		: message(form, { outcome: { kind: "unavailable" }, resend: "idle" }, { status: 500 });
}

export const actions: Actions = {
	signUp: async ({ locals, request }) => requestSignUp(locals.api, await request.formData(), false),
	resend: async ({ locals, request }) => requestSignUp(locals.api, await request.formData(), true),
};

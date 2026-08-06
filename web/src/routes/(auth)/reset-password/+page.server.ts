import { fail } from "@sveltejs/kit";
import { message, setError, superValidate, type Infer } from "sveltekit-superforms";
import { zod4 } from "sveltekit-superforms/adapters";
import {
	emailMessage,
	passwordMessage,
	resetLinkFailure,
	resetRequestFailure,
	resetSent,
} from "$lib/auth/password-reset";
import { newPasswordSchema, resetRequestSchema } from "$lib/auth/reset-password-schema";
import type { PasswordReset } from "$lib/auth/types";
import type { Actions, PageServerLoad } from "./$types";

type RequestForm = Infer<typeof resetRequestSchema>;
type PasswordForm = Infer<typeof newPasswordSchema>;

const requestFormId = "reset-request-form";
const passwordFormId = "new-password-form";

export const load: PageServerLoad = async ({ url }) => {
	const token = url.searchParams.get("token");

	return {
		token,
		reset: (token ? { kind: "form" } : { kind: "request" }) as PasswordReset,
		requestForm: await superValidate<RequestForm, PasswordReset>(zod4(resetRequestSchema), {
			id: requestFormId,
		}),
		passwordForm: await superValidate<PasswordForm, PasswordReset>(zod4(newPasswordSchema), {
			id: passwordFormId,
		}),
	};
};

export const actions: Actions = {
	request: async ({ locals, request }) => {
		const form = await superValidate<RequestForm, PasswordReset>(request, zod4(resetRequestSchema), {
			id: requestFormId,
		});

		if (!form.valid) return fail(400, { form });

		const { data, error } = await locals.api.POST("/auth/password-reset", {
			body: { email: form.data.email },
		});

		if (!error) return message(form, resetSent(form.data.email, data, new Date()));

		const failure = resetRequestFailure(error);

		if (failure) return message(form, failure, { status: 400 });

		for (const field of error.errors ?? []) {
			if (field.field === "email") setError(form, "email", emailMessage(field.code));
		}

		return fail(400, { form });
	},

	reset: async ({ locals, request }) => {
		const body = await request.formData();
		const form = await superValidate<PasswordForm, PasswordReset>(body, zod4(newPasswordSchema), {
			id: passwordFormId,
		});

		const token = String(body.get("token") ?? "");
		const password = form.data.password;

		form.data.password = "";

		if (!form.valid) return fail(400, { form });
		if (!token) return message(form, { kind: "link_expired" }, { status: 400 });

		const { error } = await locals.api.POST("/auth/password-reset/confirm", {
			body: { token, password },
		});

		if (!error) return message(form, { kind: "changed" });

		const outcome = resetLinkFailure(error);

		if (outcome) return message(form, outcome, { status: 400 });

		for (const field of error.errors ?? []) {
			if (field.field === "password") setError(form, "password", passwordMessage(field.code));
		}

		return fail(400, { form });
	},
};

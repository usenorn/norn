import { fail, type RequestEvent } from "@sveltejs/kit";
import { message, setError, superValidate, type Infer, type SuperValidated } from "sveltekit-superforms";
import { zod4 } from "sveltekit-superforms/adapters";
import { telegramFailure, telegramFailureTitle, type TelegramOutcome } from "./telegram";
import { telegramConnectFormId, telegramConnectSchema } from "./telegram-schema";

export type TelegramConnectForm = SuperValidated<Infer<typeof telegramConnectSchema>, TelegramOutcome>;

export function telegramConnectForm(workspaceId: string, agentId: string): Promise<TelegramConnectForm> {
	return superValidate<Infer<typeof telegramConnectSchema>, TelegramOutcome>(
		{ workspaceId, agentId, token: "" },
		zod4(telegramConnectSchema),
		{ id: telegramConnectFormId, errors: false }
	);
}

export async function connectTelegram({ locals, request }: RequestEvent) {
	const form = await superValidate<Infer<typeof telegramConnectSchema>, TelegramOutcome>(
		request,
		zod4(telegramConnectSchema),
		{ id: telegramConnectFormId }
	);

	const token = form.data.token;

	form.data.token = "";

	if (!form.valid) return fail(400, { form });

	const { data, error, response } = await locals.api.PUT("/workspaces/{workspaceId}/agents/{agentId}/telegram", {
		params: { path: { workspaceId: form.data.workspaceId, agentId: form.data.agentId } },
		body: { token },
	});

	if (data) return message(form, { kind: "connected" });

	const failure = telegramFailure(error, response.status);

	if (failure.kind === "refused" && failure.code === "token_rejected") {
		setError(form, "token", telegramFailureTitle(failure));
	}

	return message(form, { kind: "failed", failure }, { status: failure.kind === "forbidden" ? 403 : 400 });
}

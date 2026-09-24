import { fail } from "@sveltejs/kit";
import type { Client } from "openapi-fetch";
import { message, setError, superValidate, type Infer } from "sveltekit-superforms";
import { zod4 } from "sveltekit-superforms/adapters";
import { keys } from "$lib/api/keys";
import type { paths } from "$lib/api/dashboard.gen";
import { reachInstance } from "$lib/auth/instance";
import { aiProviderSchema } from "$lib/ai-provider/ai-provider-schema";
import {
	aiProviderFailure,
	failureTitle,
	type AiProviderModels,
	type AiProviderOutcome,
	type AiProviderView,
} from "$lib/ai-provider/ai-provider";
import type { Actions, PageServerLoad } from "./$types";

type AiProviderForm = Infer<typeof aiProviderSchema>;

const formId = "ai-provider-form";

type AiProviderField = "apiKey" | "defaultModel" | "provider" | "baseUrl" | "allowPrivateAddress";

const fieldMessages: Record<AiProviderField, Partial<Record<string, string>>> = {
	apiKey: {
		required: "Paste the API key from your provider account.",
		too_long: "That is longer than any key the provider issues.",
	},
	defaultModel: {
		required: "Choose the model a test request is sent to.",
		too_long: "That is longer than any model name the provider uses.",
	},
	provider: {
		unsupported_value: "Norn does not support that provider yet.",
	},
	baseUrl: {
		malformed: "Enter a full http or https address, for example https://gateway.example.com/v1.",
		too_long: "That address is too long.",
	},
	allowPrivateAddress: {
		unsupported_value: "This instance only connects to public endpoints.",
	},
};

const keyAgainMessage = "Paste the key again. A stored key is never sent to a changed endpoint.";

function isFormField(field: string): field is AiProviderField {
	return field in fieldMessages;
}

export const load: PageServerLoad = async ({ depends, route, locals, parent, url }) => {
	depends(keys.page(route.id));

	const { workspace } = await parent();

	const [view, instance] = await Promise.all([
		readView(locals.api, workspace.id),
		reachInstance(locals.api, url),
	]);

	const configured = view.kind === "configured" ? view.provider : undefined;

	return {
		view,
		selfHosted: instance.selfHosted,
		form: await superValidate<AiProviderForm, AiProviderOutcome>(
			{
				provider: configured?.provider ?? "openai",
				baseUrl: configured?.baseUrl ?? "",
				allowPrivateAddress: configured?.allowPrivateAddress ?? false,
				defaultModel: configured?.defaultModel ?? "",
				keyStored: configured !== undefined,
			},
			zod4(aiProviderSchema),
			{ id: formId, errors: false }
		),
	};
};

async function readView(api: Client<paths>, workspaceId: string): Promise<AiProviderView> {
	const params = { path: { workspaceId } };

	const { data: provider, error } = await api.GET("/workspaces/{workspaceId}/ai-provider", { params });

	if (error) {
		if (error.status === 404) return { kind: "unconfigured" };
		if (error.status === 403) return { kind: "forbidden" };

		return { kind: "unavailable" };
	}

	if (!provider) return { kind: "unavailable" };

	return { kind: "configured", provider, models: await readModels(api, workspaceId) };
}

async function readModels(api: Client<paths>, workspaceId: string): Promise<AiProviderModels> {
	const { data, error } = await api.GET("/workspaces/{workspaceId}/ai-provider/models", {
		params: { path: { workspaceId } },
	});

	if (error) return { kind: "failed", failure: aiProviderFailure(error) };

	return data ? { kind: "listed", models: data.models } : { kind: "failed", failure: { kind: "unavailable" } };
}

export const actions: Actions = {
	save: async ({ locals, request }) => {
		const body = await request.formData();
		const form = await superValidate<AiProviderForm, AiProviderOutcome>(body, zod4(aiProviderSchema), {
			id: formId,
		});

		const apiKey = form.data.apiKey;

		form.data.apiKey = "";

		if (!form.valid) return fail(400, { form });

		const workspaceId = String(body.get("workspaceId") ?? "");

		const { data, error } = await locals.api.PUT("/workspaces/{workspaceId}/ai-provider", {
			params: { path: { workspaceId } },
			body: {
				provider: form.data.provider,
				baseUrl: form.data.baseUrl,
				allowPrivateAddress: form.data.allowPrivateAddress,
				apiKey: apiKey || undefined,
				defaultModel: form.data.defaultModel || undefined,
			},
		});

		if (data) {
			form.data.keyStored = true;
			form.data.defaultModel = data.defaultModel;
			form.data.baseUrl = data.baseUrl;
			form.data.allowPrivateAddress = data.allowPrivateAddress;

			return message(form, { kind: "saved" });
		}

		if (!error) return message(form, { kind: "failed", failure: { kind: "unavailable" } }, { status: 500 });

		if (error.errors?.length) {
			for (const field of error.errors) {
				if (!isFormField(field.field)) continue;

				const text =
					field.field === "apiKey" && field.code === "required" && form.data.keyStored
						? keyAgainMessage
						: fieldMessages[field.field][field.code];

				if (text) setError(form, field.field, text);
			}

			return fail(400, { form });
		}

		const failure = aiProviderFailure(error);

		if (failure.kind === "provider" && failure.code === "key_rejected") {
			setError(form, "apiKey", failureTitle(failure, form.data.provider));
		}

		if (failure.kind === "provider" && failure.code === "model_unavailable") {
			setError(form, "defaultModel", failureTitle(failure, form.data.provider));
		}

		return message(form, { kind: "failed", failure }, { status: failure.kind === "forbidden" ? 403 : 400 });
	},
};

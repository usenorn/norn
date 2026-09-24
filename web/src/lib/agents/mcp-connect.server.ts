import { fail, redirect, type RequestEvent } from "@sveltejs/kit";
import { message, superValidate, type Infer, type SuperValidated } from "sveltekit-superforms";
import { zod4 } from "sveltekit-superforms/adapters";
import { capabilityFailure, type AgentCapabilityFailure } from "./agent-capabilities";
import { mcpConnectFormId, mcpConnectSchema } from "./agent-capability-schemas";

export type McpConnectForm = SuperValidated<Infer<typeof mcpConnectSchema>, AgentCapabilityFailure>;

export function mcpConnectForm(workspaceId: string, returnTo: string): Promise<McpConnectForm> {
	return superValidate<Infer<typeof mcpConnectSchema>, AgentCapabilityFailure>(
		{ workspaceId, serverId: "", returnTo },
		zod4(mcpConnectSchema),
		{ id: mcpConnectFormId, errors: false }
	);
}

export async function connectMcpServer({ locals, request }: RequestEvent) {
	const form = await superValidate<Infer<typeof mcpConnectSchema>, AgentCapabilityFailure>(
		request,
		zod4(mcpConnectSchema),
		{ id: mcpConnectFormId }
	);

	if (!form.valid) return fail(400, { form });

	const { data, error, response } = await locals.api.POST(
		"/workspaces/{workspaceId}/agent-mcp-servers/{serverId}/connect",
		{
			params: { path: { workspaceId: form.data.workspaceId, serverId: form.data.serverId } },
			body: { returnTo: form.data.returnTo },
		}
	);

	if (error || !data) {
		const failure = capabilityFailure(error, response.status);

		return message(form, failure, { status: failure.kind === "forbidden" ? 403 : 400 });
	}

	redirect(303, data.authorizationUrl);
}

import { z } from "zod";

export const agentCapabilityKinds = ["skill", "mcp"] as const;
export const agentMCPTransports = ["stdio", "remote"] as const;
export const agentMCPAuthMethods = ["none", "bearer"] as const;
export const agentMCPRemoteProtocols = ["http:", "https:"] as const;

export const agentCapabilityDraftSchema = z
	.object({
		kind: z.enum(agentCapabilityKinds).default("skill"),
		name: z.string().trim().default(""),
		source: z.string().trim().default(""),
		transport: z.enum(agentMCPTransports).default("remote"),
		command: z.string().trim().default(""),
		args: z.string().default(""),
		url: z.string().trim().default(""),
		auth: z.enum(agentMCPAuthMethods).default("none"),
		secret: z.string().default(""),
	})
	.superRefine((draft, context) => {
		if ((draft.kind === "skill" || draft.kind === "mcp") && draft.name === "") {
			context.addIssue({ code: "custom", path: ["name"], message: "Name this capability." });
		}

		if (draft.kind === "skill" && draft.source === "") {
			context.addIssue({
				code: "custom",
				path: ["source"],
				message: "Enter a source URL or repository path.",
			});
		}

		if (draft.kind === "mcp") {
			if (draft.transport === "stdio") {
				if (draft.command === "") {
					context.addIssue({ code: "custom", path: ["command"], message: "Enter a command." });
				}
			} else {
				const url = z.string().url().safeParse(draft.url);
				const protocol = url.success ? new URL(url.data).protocol : "";

				if (!url.success || !agentMCPRemoteProtocols.some((allowed) => allowed === protocol)) {
					context.addIssue({
						code: "custom",
						path: ["url"],
						message: "Enter an HTTP or HTTPS server URL.",
					});
				}

				if (draft.auth === "bearer" && draft.secret.trim() === "") {
					context.addIssue({ code: "custom", path: ["secret"], message: "Enter the bearer token." });
				}
			}
		}
	});

export type AgentCapabilityDraftInput = z.infer<typeof agentCapabilityDraftSchema>;
export type AgentCapabilityKind = AgentCapabilityDraftInput["kind"];
export type AgentMCPTransport = AgentCapabilityDraftInput["transport"];
export type AgentMCPAuthMethod = AgentCapabilityDraftInput["auth"];

export type AgentSkillDraft = {
	kind: "skill";
	name: AgentCapabilityDraftInput["name"];
	source: AgentCapabilityDraftInput["source"];
};

export type AgentMCPStdioServerDraft = {
	kind: "mcp";
	name: AgentCapabilityDraftInput["name"];
	transport: "stdio";
	command: AgentCapabilityDraftInput["command"];
	args: string[];
};

export type AgentMCPRemoteServerDraft = {
	kind: "mcp";
	name: AgentCapabilityDraftInput["name"];
	transport: "remote";
	url: AgentCapabilityDraftInput["url"];
	auth: AgentMCPAuthMethod;
};

export type AgentMCPServerDraft = AgentMCPStdioServerDraft | AgentMCPRemoteServerDraft;
export type AgentCapabilityDraft = AgentSkillDraft | AgentMCPServerDraft;

export type AgentCapabilities =
	| { kind: "loading" }
	| { kind: "empty" }
	| {
			kind: "ready";
			skills: AgentSkillDraft[];
			mcpServers: AgentMCPServerDraft[];
		}
	| { kind: "unavailable" };

export function capabilityDraft(input: AgentCapabilityDraftInput): AgentCapabilityDraft {
	switch (input.kind) {
		case "skill":
			return { kind: "skill", name: input.name, source: input.source };
		case "mcp":
			return input.transport === "stdio"
				? {
						kind: "mcp",
						name: input.name,
						transport: "stdio",
						command: input.command,
						args: input.args
							.split("\n")
							.map((argument) => argument.trim())
							.filter(Boolean),
					}
				: {
						kind: "mcp",
						name: input.name,
						transport: "remote",
						url: input.url,
						auth: input.auth,
					};
	}
}

export function withCapability(
	capabilities: AgentCapabilities,
	draft: AgentCapabilityDraft
): AgentCapabilities {
	const ready =
		capabilities.kind === "ready"
			? capabilities
			: { kind: "ready" as const, skills: [], mcpServers: [] };

	switch (draft.kind) {
		case "skill":
			return { ...ready, skills: [...ready.skills, draft] };
		case "mcp":
			return { ...ready, mcpServers: [...ready.mcpServers, draft] };
	}
}

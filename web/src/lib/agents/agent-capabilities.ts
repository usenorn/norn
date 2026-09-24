import type { components } from "$lib/api/dashboard.gen";

export type AgentSkill = components["schemas"]["AgentSkill"];
export type AgentMcpServer = components["schemas"]["AgentMcpServer"];
export type AgentMcpTransport = components["schemas"]["AgentMcpTransport"];
export type AgentMcpAuth = components["schemas"]["AgentMcpAuth"];
export type AgentLibrary = components["schemas"]["AgentLibrary"];
export type AgentLibrarySkill = components["schemas"]["AgentLibrarySkill"];
export type AgentLibraryMcpServer = components["schemas"]["AgentLibraryMcpServer"];
export type SkillSourceDiscovery = components["schemas"]["SkillSourceDiscovery"];
export type McpRegistryEntry = components["schemas"]["McpRegistryEntry"];
export type McpServerTemplate = components["schemas"]["McpServerTemplate"];

type ConflictCode = components["schemas"]["AgentCapabilityConflictProblem"]["code"];
type UpstreamCode = components["schemas"]["AgentCapabilityUpstreamProblem"]["code"];

export type AgentCapabilityKind = "skill" | "mcp";

export type AgentCapabilities =
	| { kind: "loading" }
	| { kind: "empty" }
	| { kind: "ready"; skills: AgentSkill[]; mcpServers: AgentMcpServer[] }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

export type AgentLibraryListing =
	| { kind: "loading" }
	| { kind: "empty" }
	| { kind: "ready"; library: AgentLibrary }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

export type AgentCapabilityFailure =
	| { kind: "conflict"; code: ConflictCode }
	| { kind: "upstream"; code: UpstreamCode }
	| { kind: "validation" }
	| { kind: "missing" }
	| { kind: "forbidden" }
	| { kind: "sealing_unavailable" }
	| { kind: "unavailable" };

export type McpSignIn =
	| { kind: "not_needed" }
	| { kind: "signed_out" }
	| { kind: "connected"; issuer: string; expiresAt?: string }
	| { kind: "expired"; issuer: string }
	| { kind: "failed"; issuer: string };

export type McpConnectOutcome =
	| { kind: "none" }
	| { kind: "connected" }
	| { kind: "refused" }
	| { kind: "expired" }
	| { kind: "removed" }
	| { kind: "failed"; reference?: string };

export type CapabilityMutation =
	| { kind: "idle" }
	| { kind: "working"; id: string }
	| { kind: "failed"; failure: AgentCapabilityFailure };

const conflictMessages: Record<ConflictCode, string> = {
	skill_name_taken: "This agent already has a skill with that name. Rename one in its SKILL.md.",
	mcp_server_name_taken: "This agent already has an MCP server with that name.",
	skill_limit_reached: "No more skills fit here. Remove one first.",
	mcp_server_limit_reached: "No more MCP servers fit here. Remove one first.",
	already_attached: "This agent already uses that library item.",
	not_attached: "This agent no longer uses that library item.",
	not_library: "Only library items can be shared with an agent.",
	skill_imported: "This skill is imported. Update it from its source instead.",
	skill_not_imported: "This skill was written here, so there is no source to update it from.",
	oauth_not_configured: "This server does not sign in with OAuth.",
	oauth_unsupported:
		"This server does not publish how to sign in to it. Add a client ID from its provider and try again.",
	oauth_refused: "The provider refused the sign-in.",
	destination_refused: "This instance will not connect to that address.",
};

const upstreamMessages: Record<UpstreamCode, string> = {
	skill_source_unreachable: "GitHub could not be reached. Try again in a moment.",
	registry_unreachable: "The MCP registry could not be reached. Try again in a moment.",
	mcp_server_unreachable: "The MCP server could not be reached. Check its address.",
};

export function capabilityFailure(error: unknown, status: number): AgentCapabilityFailure {
	if (status === 403) return { kind: "forbidden" };
	if (status === 404) return { kind: "missing" };
	if (status === 422) return { kind: "validation" };

	const code = error && typeof error === "object" && "code" in error ? String(error.code) : "";

	if (status === 409 && code in conflictMessages) return { kind: "conflict", code: code as ConflictCode };
	if (status === 502 && code in upstreamMessages) return { kind: "upstream", code: code as UpstreamCode };
	if (status === 503 && code === "agent_capability_sealing_unavailable") return { kind: "sealing_unavailable" };

	return { kind: "unavailable" };
}

export function capabilityFailureMessage(failure: AgentCapabilityFailure): string {
	switch (failure.kind) {
		case "conflict":
			return conflictMessages[failure.code];
		case "upstream":
			return upstreamMessages[failure.code];
		case "validation":
			return "Check the highlighted fields.";
		case "missing":
			return "Nothing was found there. Check the address and try again.";
		case "forbidden":
			return "You may not change this. Ask the agent's owner or a workspace administrator.";
		case "sealing_unavailable":
			return "This instance has no encryption key, so secrets cannot be stored. Ask your administrator to set one.";
		case "unavailable":
			return "That did not work. Check your connection and try again.";
	}
}

export function listedCapabilities(skills: AgentSkill[], mcpServers: AgentMcpServer[]): AgentCapabilities {
	if (skills.length === 0 && mcpServers.length === 0) return { kind: "empty" };

	return { kind: "ready", skills, mcpServers };
}

export function listedLibrary(library: AgentLibrary): AgentLibraryListing {
	if (library.skills.length === 0 && library.mcpServers.length === 0) return { kind: "empty" };

	return { kind: "ready", library };
}

export function mcpSignIn(server: AgentMcpServer): McpSignIn {
	if (server.auth !== "oauth") return { kind: "not_needed" };

	const connection = server.connection;
	if (!connection) return { kind: "signed_out" };

	switch (connection.status) {
		case "connected":
			return { kind: "connected", issuer: connection.issuer, expiresAt: connection.expiresAt };
		case "expired":
			return { kind: "expired", issuer: connection.issuer };
		case "failed":
			return { kind: "failed", issuer: connection.issuer };
	}
}

export function mcpServerTarget(server: Pick<AgentMcpServer, "transport" | "command" | "args" | "url">): string {
	return server.transport === "stdio" ? [server.command, ...server.args].join(" ") : server.url;
}

export function mcpServerSummary(server: AgentMcpServer): string {
	const transport =
		server.transport === "stdio" ? "Runs on the runner" : server.transport === "sse" ? "Remote · SSE" : "Remote · HTTP";

	if (server.auth === "oauth") return `${transport} · signs in with OAuth`;
	if (server.auth === "headers") return `${transport} · ${plural(server.headerKeys.length, "header")}`;
	if (server.envKeys.length > 0) return `${transport} · ${plural(server.envKeys.length, "variable")}`;

	return transport;
}

export function skillSummary(skill: AgentSkill): string {
	if (skill.origin) {
		const path = skill.origin.path ? `/${skill.origin.path}` : "";

		return `${skill.origin.repository}${path} @ ${skill.origin.revision.slice(0, 7)}`;
	}

	return `Written here · ${plural(skill.fileCount, "file")}`;
}

function plural(count: number, noun: string): string {
	return `${count} ${noun}${count === 1 ? "" : "s"}`;
}

export function connectOutcome(url: URL): McpConnectOutcome {
	switch (url.searchParams.get("connection")) {
		case "connected":
			return { kind: "connected" };
		case "refused":
			return { kind: "refused" };
		case "expired":
			return { kind: "expired" };
		case "removed":
			return { kind: "removed" };
		case "failed":
			return { kind: "failed", reference: url.searchParams.get("reference") ?? undefined };
		default:
			return { kind: "none" };
	}
}

export function agentLibraryPath(workspace: string): string {
	return `/${workspace}/settings/agent-library`;
}

export function unattached<T extends { id: string }>(library: T[], attached: { id: string }[]): T[] {
	const taken = new Set(attached.map((item) => item.id));

	return library.filter((item) => !taken.has(item.id));
}

export type SkillAction = "pull" | "edit" | "detach" | "delete";
export type CapabilityDialogPreview = "skill" | "mcp" | "picker-skill" | "picker-mcp";
export type SkillDialogMode = { kind: "add" } | { kind: "rewrite"; skill: AgentSkill };
export type McpServerDialogMode = { kind: "add" } | { kind: "edit"; server: AgentMcpServer };
export type McpServerAction = "edit" | "disconnect" | "detach" | "delete";

export type SkillEntry = { skill: AgentSkill; shared: boolean; usedBy?: string[] };
export type McpServerEntry = { server: AgentMcpServer; shared: boolean; usedBy?: string[] };

export type CapabilityRows =
	| { kind: "loading" }
	| { kind: "forbidden" }
	| { kind: "unavailable" }
	| { kind: "listed"; skills: SkillEntry[]; servers: McpServerEntry[] };

export function agentRows(capabilities: AgentCapabilities): CapabilityRows {
	switch (capabilities.kind) {
		case "loading":
		case "forbidden":
		case "unavailable":
			return capabilities;
		case "empty":
			return { kind: "listed", skills: [], servers: [] };
		case "ready":
			return {
				kind: "listed",
				skills: capabilities.skills.map((skill) => ({ skill, shared: skill.library })),
				servers: capabilities.mcpServers.map((server) => ({ server, shared: server.library })),
			};
	}
}

export function libraryRows(listing: AgentLibraryListing, agentName: (id: string) => string): CapabilityRows {
	switch (listing.kind) {
		case "loading":
		case "forbidden":
		case "unavailable":
			return listing;
		case "empty":
			return { kind: "listed", skills: [], servers: [] };
		case "ready":
			return {
				kind: "listed",
				skills: listing.library.skills.map((entry) => ({
					skill: entry.skill,
					shared: false,
					usedBy: entry.agentIds.map(agentName),
				})),
				servers: listing.library.mcpServers.map((entry) => ({
					server: entry.server,
					shared: false,
					usedBy: entry.agentIds.map(agentName),
				})),
			};
	}
}

export type FieldProblem = { field: string; code: string };

const skillFieldMessages: Record<string, Record<string, string>> = {
	source: {
		required: "Paste a GitHub repository, a skills.sh address, or an npx skills add command.",
		malformed:
			"Norn reads skills from GitHub repositories, github.com and skills.sh addresses, and npx skills add commands.",
		too_long: "That source is too long.",
	},
	path: { required: "Pick the skill to add." },
	name: {
		required: "The SKILL.md frontmatter needs a name.",
		malformed: "The name in the frontmatter must use lowercase letters, digits, and single dashes.",
		too_long: "The name in the frontmatter is longer than 64 characters.",
	},
	description: {
		required: "The SKILL.md frontmatter needs a description.",
		too_long: "The description is longer than 1024 characters.",
	},
	instructions: {
		required: "Add the instructions below the frontmatter.",
		malformed: "SKILL.md must start with frontmatter between two --- lines.",
	},
	bundle: {
		required: "The skill has no files.",
		too_long: "The skill is larger than 5 MB or holds more than 200 files.",
		malformed: "The skill holds a file outside its own folder.",
	},
	archive: {
		malformed: "That file is not a zip.",
		too_long: "The zip is larger than the skill limit.",
	},
};

export function skillFieldMessage(problem: FieldProblem): string {
	return skillFieldMessages[problem.field]?.[problem.code] ?? "Check this skill and try again.";
}

const serverFieldMessages: Record<string, Record<string, string>> = {
	name: {
		required: "Name the server.",
		malformed: "Use lowercase letters, digits, and single dashes.",
		too_long: "Keep the name under 64 characters.",
		taken: "norn is the runner's own server. Pick another name.",
	},
	command: { required: "Enter the command that starts the server.", malformed: "That command cannot be run." },
	args: { malformed: "An argument is too long.", too_long: "There are too many arguments." },
	url: {
		required: "Enter an http or https address.",
		malformed: "Enter an http or https address.",
		too_long: "That address is too long.",
	},
	env: {
		required: "Enter a value for each new variable.",
		malformed: "Use letters, digits, and underscores, and keep values on one line.",
		too_long: "There are too many variables.",
	},
	headers: {
		required: "Add the header the server expects, with a value.",
		malformed: "A header name or value is not allowed.",
		too_long: "There are too many headers.",
	},
	oauthClientId: { required: "A client secret needs its client ID.", too_long: "That client ID is too long." },
};

export function serverFieldMessage(problem: FieldProblem): string {
	return serverFieldMessages[problem.field]?.[problem.code] ?? "Check this field.";
}

export function fieldProblems(error: unknown): FieldProblem[] {
	if (!error || typeof error !== "object" || !("errors" in error) || !Array.isArray(error.errors)) return [];

	return error.errors.filter(
		(problem): problem is FieldProblem =>
			typeof problem === "object" && problem !== null && "field" in problem && "code" in problem
	);
}

import { z } from "zod";
import type { components } from "$lib/api/dashboard.gen";
import type { AgentMcpServer, McpRegistryEntry, McpServerTemplate } from "./agent-capabilities";

type AgentMcpServerRequest = components["schemas"]["AgentMcpServerRequest"];

export const capabilityNamePattern = /^[a-z0-9]+(-[a-z0-9]+)*$/;
const envKeyPattern = /^[A-Za-z_][A-Za-z0-9_]*$/;
const headerNamePattern = /^[!#$%&'*+.^_`|~0-9A-Za-z-]+$/;
const reservedServerName = "norn";
export const skillArchiveMaxBytes = 3 << 20;

export const skillSourceSchema = z.object({
	source: z
		.string()
		.trim()
		.min(1, "Paste a GitHub repository, a skills.sh address, or an npx skills add command.")
		.max(512, "That source is too long."),
	path: z.string().default(""),
});

export const skillWriteSchema = z
	.object({
		instructions: z.string().max(262144, "A SKILL.md this long will not fit. Upload the folder as a zip.").default(""),
		archive: z
			.instanceof(File)
			.refine((file) => file.size <= skillArchiveMaxBytes, "The zip is larger than 3 MB.")
			.optional(),
	})
	.superRefine((draft, context) => {
		if (draft.instructions.trim() === "" && !draft.archive) {
			context.addIssue({
				code: "custom",
				path: ["instructions"],
				message: "Write the SKILL.md, or upload the skill folder as a zip.",
			});
		}
	});

const variableSchema = z.object({
	key: z.string().trim(),
	value: z.string(),
	stored: z.boolean().default(false),
});

export const mcpServerSchema = z
	.object({
		name: z
			.string()
			.trim()
			.min(1, "Name the server.")
			.max(64, "Keep the name under 64 characters.")
			.regex(capabilityNamePattern, "Use lowercase letters, digits, and single dashes."),
		transport: z.enum(["stdio", "http", "sse"]).default("http"),
		command: z.string().trim().default(""),
		args: z.string().default(""),
		url: z.string().trim().default(""),
		auth: z.enum(["none", "headers", "oauth"]).default("none"),
		env: z.array(variableSchema).default([]),
		headers: z.array(variableSchema).default([]),
		oauthClientId: z.string().trim().default(""),
		oauthClientSecret: z.string().default(""),
		oauthClientSecretStored: z.boolean().default(false),
		registryName: z.string().default(""),
		registryVersion: z.string().default(""),
	})
	.superRefine((draft, context) => {
		if (draft.name === reservedServerName) {
			context.addIssue({ code: "custom", path: ["name"], message: "norn is the runner's own server. Pick another name." });
		}

		if (draft.transport === "stdio") {
			if (draft.command === "") {
				context.addIssue({ code: "custom", path: ["command"], message: "Enter the command that starts the server." });
			}

			checkVariables(draft.env, "env", envKeyPattern, "Use letters, digits, and underscores.", context);

			return;
		}

		if (!isHttpUrl(draft.url)) {
			context.addIssue({ code: "custom", path: ["url"], message: "Enter an http or https address." });
		}

		if (draft.auth === "headers") {
			if (draft.headers.filter((header) => header.key !== "").length === 0) {
				context.addIssue({ code: "custom", path: ["headers"], message: "Add the header the server expects." });
			}

			checkVariables(draft.headers, "headers", headerNamePattern, "That is not a header name.", context);
		}

		if (draft.auth === "oauth" && draft.oauthClientSecret !== "" && draft.oauthClientId === "") {
			context.addIssue({ code: "custom", path: ["oauthClientId"], message: "A client secret needs its client ID." });
		}
	});

function checkVariables(
	variables: z.infer<typeof variableSchema>[],
	field: "env" | "headers",
	pattern: RegExp,
	malformed: string,
	context: z.RefinementCtx
) {
	variables.forEach((variable, index) => {
		if (variable.key === "") return;

		if (!pattern.test(variable.key)) {
			context.addIssue({ code: "custom", path: [field, index, "key"], message: malformed });
		}

		if (!variable.stored && variable.value === "") {
			context.addIssue({ code: "custom", path: [field, index, "value"], message: "Enter a value." });
		}
	});
}

function isHttpUrl(value: string): boolean {
	const parsed = URL.canParse(value) ? new URL(value) : null;

	return parsed !== null && (parsed.protocol === "https:" || parsed.protocol === "http:");
}

export const mcpConnectFormId = "mcp-connect-form";

export const mcpConnectSchema = z.object({
	workspaceId: z.uuid(),
	serverId: z.uuid(),
	returnTo: z.string().startsWith("/"),
});

export type McpServerInput = z.infer<typeof mcpServerSchema>;

export function emptyMcpServer(): McpServerInput {
	return {
		name: "",
		transport: "http",
		command: "",
		args: "",
		url: "",
		auth: "none",
		env: [],
		headers: [],
		oauthClientId: "",
		oauthClientSecret: "",
		oauthClientSecretStored: false,
		registryName: "",
		registryVersion: "",
	};
}

export function mcpServerInput(server: AgentMcpServer): McpServerInput {
	return {
		...emptyMcpServer(),
		name: server.name,
		transport: server.transport,
		command: server.command,
		args: server.args.join("\n"),
		url: server.url,
		auth: server.auth,
		env: server.envKeys.map((key) => ({ key, value: "", stored: true })),
		headers: server.headerKeys.map((key) => ({ key, value: "", stored: true })),
		oauthClientId: server.oauthClientId,
		oauthClientSecretStored: server.oauthClientId !== "",
		registryName: server.registryName ?? "",
		registryVersion: server.registryVersion ?? "",
	};
}

export function registryServerName(entry: McpRegistryEntry): string {
	const last = entry.name.split("/").pop() ?? entry.name;
	const slug = last
		.toLowerCase()
		.replace(/[^a-z0-9]+/g, "-")
		.replace(/-mcp(-server)?$/, "")
		.replace(/^mcp-/, "")
		.replace(/^-+|-+$/g, "");

	return slug.slice(0, 64) || "server";
}

export function registryServerInput(entry: McpRegistryEntry, template: McpServerTemplate): McpServerInput {
	const secretHeader = template.headers.some((header) => header.key.toLowerCase() === "authorization");

	return {
		...emptyMcpServer(),
		name: registryServerName(entry),
		transport: template.transport,
		command: template.command,
		args: template.args.join("\n"),
		url: template.url,
		auth: template.transport === "stdio" ? "none" : template.headers.length > 0 ? "headers" : "oauth",
		env: template.env.map((variable) => ({ key: variable.key, value: variable.default ?? "", stored: false })),
		headers: template.headers.map((header) => ({
			key: header.key,
			value: secretHeader && header.key.toLowerCase() === "authorization" ? "" : (header.default ?? ""),
			stored: false,
		})),
		registryName: entry.name,
		registryVersion: entry.version,
	};
}

function variables(entries: z.infer<typeof variableSchema>[]): Record<string, string> | undefined {
	const present = entries.filter((entry) => entry.key !== "");
	if (present.length === 0) return undefined;

	return Object.fromEntries(present.map((entry) => [entry.key, entry.value]));
}

export function mcpServerRequest(input: McpServerInput): AgentMcpServerRequest {
	const remote = input.transport !== "stdio";

	return {
		name: input.name,
		transport: input.transport,
		command: remote ? undefined : input.command,
		args: remote
			? undefined
			: input.args
					.split("\n")
					.map((argument) => argument.trim())
					.filter(Boolean),
		url: remote ? input.url : undefined,
		auth: remote ? input.auth : "none",
		env: remote ? undefined : variables(input.env),
		headers: remote && input.auth === "headers" ? variables(input.headers) : undefined,
		oauthClientId: remote && input.auth === "oauth" ? input.oauthClientId || undefined : undefined,
		oauthClientSecret: remote && input.auth === "oauth" ? input.oauthClientSecret || undefined : undefined,
		registryName: input.registryName || undefined,
		registryVersion: input.registryVersion || undefined,
	};
}

export async function archiveBase64(file: File): Promise<string> {
	const bytes = new Uint8Array(await file.arrayBuffer());
	let binary = "";

	for (let offset = 0; offset < bytes.length; offset += 0x8000) {
		binary += String.fromCharCode(...bytes.subarray(offset, offset + 0x8000));
	}

	return btoa(binary);
}

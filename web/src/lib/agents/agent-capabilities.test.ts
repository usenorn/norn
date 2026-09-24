import { describe, expect, it } from "vitest";
import {
	capabilityFailure,
	connectOutcome,
	libraryRows,
	mcpSignIn,
	skillSummary,
	unattached,
	type AgentMcpServer,
	type AgentSkill,
} from "./agent-capabilities";
import {
	mcpServerRequest,
	mcpServerSchema,
	registryServerInput,
	registryServerName,
	emptyMcpServer,
} from "./agent-capability-schemas";

function server(fields: Partial<AgentMcpServer>): AgentMcpServer {
	return {
		id: "00000000-0000-4000-8000-000000000c01",
		name: "linear",
		transport: "http",
		command: "",
		args: [],
		url: "https://mcp.linear.app/mcp",
		auth: "oauth",
		envKeys: [],
		headerKeys: [],
		oauthClientId: "",
		library: false,
		createdAt: "2026-09-22T09:58:00Z",
		updatedAt: "2026-09-22T09:58:00Z",
		...fields,
	};
}

describe("reading a refusal", () => {
	it("keeps the conflict code so the screen can say which fix applies", () => {
		expect(capabilityFailure({ code: "skill_name_taken" }, 409)).toEqual({ kind: "conflict", code: "skill_name_taken" });
	});

	it("reads an unreachable source as upstream, not as a mistake in the form", () => {
		expect(capabilityFailure({ code: "skill_source_unreachable" }, 502)).toEqual({
			kind: "upstream",
			code: "skill_source_unreachable",
		});
	});

	it("reads a missing encryption key as an instance problem", () => {
		expect(capabilityFailure({ code: "agent_capability_sealing_unavailable" }, 503)).toEqual({
			kind: "sealing_unavailable",
		});
	});

	it("falls back to unavailable for a code it does not know", () => {
		expect(capabilityFailure({ code: "something_new" }, 409)).toEqual({ kind: "unavailable" });
	});
});

describe("saying whether a server can be reached from a run", () => {
	it("needs no sign-in for a server that does not use OAuth", () => {
		expect(mcpSignIn(server({ auth: "headers" }))).toEqual({ kind: "not_needed" });
	});

	it("asks for a sign-in until one exists", () => {
		expect(mcpSignIn(server({}))).toEqual({ kind: "signed_out" });
	});

	it("carries a failed refresh through so the row can ask to sign in again", () => {
		expect(
			mcpSignIn(
				server({
					connection: {
						status: "failed",
						issuer: "https://api.notion.com",
						scopes: [],
						failure: "refresh_rejected",
						connectedAt: "2026-09-01T08:00:00Z",
					},
				})
			)
		).toEqual({ kind: "failed", issuer: "https://api.notion.com" });
	});
});

describe("coming back from a sign-in", () => {
	it("reads the outcome the callback appended", () => {
		expect(connectOutcome(new URL("https://norn.test/acme/settings/agent-library?connection=connected"))).toEqual({
			kind: "connected",
		});
		expect(
			connectOutcome(new URL("https://norn.test/acme/settings/agent-library?connection=failed&reference=01J8"))
		).toEqual({ kind: "failed", reference: "01J8" });
		expect(connectOutcome(new URL("https://norn.test/acme/settings/agent-library"))).toEqual({ kind: "none" });
	});
});

describe("describing a skill", () => {
	it("names the commit an imported skill came from", () => {
		const skill = {
			source: "github",
			fileCount: 7,
			origin: { repository: "anthropics/skills", path: "skills/pdf", ref: "", revision: "9f1c0de5b2a4c6e8" },
		} as AgentSkill;

		expect(skillSummary(skill)).toBe("anthropics/skills/skills/pdf @ 9f1c0de");
	});
});

describe("the library", () => {
	it("offers only what the agent does not already use", () => {
		expect(unattached([{ id: "a" }, { id: "b" }], [{ id: "b" }])).toEqual([{ id: "a" }]);
	});

	it("names the agents that use each item", () => {
		const rows = libraryRows(
			{
				kind: "ready",
				library: { skills: [], mcpServers: [{ server: server({ library: true }), agentIds: ["x", "y"] }] },
			},
			(id) => (id === "x" ? "triage-bot" : "an agent you do not manage")
		);

		expect(rows.kind === "listed" && rows.servers[0].usedBy).toEqual(["triage-bot", "an agent you do not manage"]);
	});
});

describe("the server form", () => {
	it("sends only what the chosen transport uses", () => {
		const request = mcpServerRequest({
			...emptyMcpServer(),
			name: "playwright",
			transport: "stdio",
			command: "npx",
			args: "-y\n\n @playwright/mcp@latest ",
			url: "https://left.over",
			env: [
				{ key: "BROWSERS", value: "/opt/browsers", stored: false },
				{ key: "", value: "", stored: false },
			],
		});

		expect(request).toEqual({
			name: "playwright",
			transport: "stdio",
			command: "npx",
			args: ["-y", "@playwright/mcp@latest"],
			url: undefined,
			auth: "none",
			env: { BROWSERS: "/opt/browsers" },
			headers: undefined,
			oauthClientId: undefined,
			oauthClientSecret: undefined,
			registryName: undefined,
			registryVersion: undefined,
		});
	});

	it("lets a stored secret stay empty but not a new one", () => {
		const stored = mcpServerSchema.safeParse({
			...emptyMcpServer(),
			name: "sentry",
			transport: "stdio",
			command: "npx",
			env: [{ key: "SENTRY_TOKEN", value: "", stored: true }],
		});
		const fresh = mcpServerSchema.safeParse({
			...emptyMcpServer(),
			name: "sentry",
			transport: "stdio",
			command: "npx",
			env: [{ key: "SENTRY_TOKEN", value: "", stored: false }],
		});

		expect(stored.success).toBe(true);
		expect(fresh.success).toBe(false);
	});

	it("refuses the runner's own name", () => {
		expect(mcpServerSchema.safeParse({ ...emptyMcpServer(), name: "norn", url: "https://x.test" }).success).toBe(false);
	});

	it("turns a registry entry into a server the person only has to review", () => {
		const entry = {
			name: "io.github.getsentry/sentry-mcp",
			title: "",
			description: "Sentry issues",
			version: "1.4.0",
			templates: [],
		};

		expect(registryServerName(entry)).toBe("sentry");

		const input = registryServerInput(entry, {
			transport: "http",
			command: "",
			args: [],
			url: "https://mcp.sentry.dev/mcp",
			env: [],
			headers: [{ key: "Authorization", required: true, secret: true, default: "Bearer {token}" }],
		});

		expect(input.auth).toBe("headers");
		expect(input.headers).toEqual([{ key: "Authorization", value: "", stored: false }]);
		expect(input.registryName).toBe("io.github.getsentry/sentry-mcp");
	});
});

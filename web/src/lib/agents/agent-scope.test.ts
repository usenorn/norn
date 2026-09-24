import { describe, expect, it } from "vitest";
import { reachSummary, type Agent } from "./agents";
import { delegatableAgents, type Member } from "./delegation";
import { registerAgentSchema } from "./register-agent-schema";

function agent(fields: Partial<Agent>): Agent {
	return {
		id: "00000000-0000-4000-8000-000000000a01",
		workspaceId: "00000000-0000-4000-8000-000000000a02",
		accountId: "00000000-0000-4000-8000-000000000a03",
		ownerAccountId: "00000000-0000-4000-8000-000000000a04",
		name: "triage-bot",
		icon: "bot",
		status: "active",
		scope: "member",
		actionLimit: 120,
		createdAt: "2026-07-02T09:00:00Z",
		...fields,
	};
}

function member(fields: Partial<Member>): Member {
	return {
		workspaceId: "00000000-0000-4000-8000-000000000a02",
		accountId: "00000000-0000-4000-8000-000000000a03",
		displayName: "triage-bot",
		role: "viewer",
		source: "manual",
		kind: "agent",
		...fields,
	};
}

describe("the agents you may hand an issue to", () => {
	it("keeps only the agents the server said are within reach", () => {
		const mine = member({ accountId: "00000000-0000-4000-8000-000000000a03" });
		const theirs = member({ accountId: "00000000-0000-4000-8000-000000000b03" });

		expect(
			delegatableAgents([mine, theirs], [agent({ accountId: mine.accountId })])
		).toEqual([mine]);
	});

	it("leaves out a deactivated agent the server still listed", () => {
		const gone = member({ deactivatedAt: "2026-08-01T09:00:00Z" });

		expect(delegatableAgents([gone], [agent({ accountId: gone.accountId })])).toEqual([]);
	});

	it("leaves out people", () => {
		const person = member({ kind: "person", accountId: "00000000-0000-4000-8000-000000000c03" });

		expect(delegatableAgents([person], [agent({ accountId: person.accountId })])).toEqual([]);
	});
});

describe("what a scope says about an agent", () => {
	it("names the project when it knows it", () => {
		expect(reachSummary(agent({ scope: "project" }), "Payments")).toBe(
			"Payments members may hand it work"
		);
	});

	it("falls back to the plain wording when the project is not loaded", () => {
		expect(reachSummary(agent({ scope: "project" }))).toBe("Project members may hand it work");
	});

	it("reads a workspace agent as open to everybody", () => {
		expect(reachSummary(agent({ scope: "workspace" }))).toBe(
			"Anybody in the workspace may hand it work"
		);
	});
});

describe("registering an agent", () => {
	it("refuses a project scope with no project chosen", () => {
		const parsed = registerAgentSchema.safeParse({
			name: "triage-bot",
			scopes: ["issue:read"],
			scope: "project",
		});

		expect(parsed.success).toBe(false);
		expect(parsed.error?.issues.some((issue) => issue.path[0] === "projectId")).toBe(true);
	});

	it("accepts a project scope carrying its project", () => {
		expect(
			registerAgentSchema.safeParse({
				name: "triage-bot",
				scopes: ["issue:read"],
				scope: "project",
				projectId: "00000000-0000-4000-8000-000000000a05",
			}).success
		).toBe(true);
	});
});

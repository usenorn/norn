import type { components } from "$lib/api/dashboard.gen";

export type MCPConnection = components["schemas"]["MCPConnection"];
export type WorkspaceMCPConnection = components["schemas"]["WorkspaceMCPConnection"];
export type MCPCapability = components["schemas"]["MCPCapability"];
export type MCPAuthorizationView = components["schemas"]["MCPAuthorizationView"];

export type ConnectionListing =
	| { kind: "loading" }
	| { kind: "empty" }
	| { kind: "ready"; connections: MCPConnection[] }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

export type WorkspaceConnectionListing =
	| { kind: "loading" }
	| { kind: "empty" }
	| { kind: "ready"; connections: WorkspaceMCPConnection[] }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

export type ConsentState =
	| { kind: "loading" }
	| { kind: "ready"; request: MCPAuthorizationView }
	| { kind: "expired" }
	| { kind: "failed" };

export type ConnectionFailure =
	| { kind: "grant_invalid" }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

export const capabilityLabel: Record<MCPCapability, string> = {
	read: "Read only",
	write: "Read and write",
};

export const capabilityLede: Record<MCPCapability, string> = {
	read: "It will be able to read everything you can see — issues, projects, cycles, and teams.",
	write:
		"It will be able to read everything you can see, and raise and change issues and " +
		"comments as you.",
};

export function reachSummary(connection: MCPConnection): string {
	if (connection.followsMembership) return "Every workspace you belong to";

	const count = connection.grants.length;

	return count === 1 ? "1 workspace" : `${count} workspaces`;
}

export function connectionFailureMessage(failure: ConnectionFailure): string {
	switch (failure.kind) {
		case "grant_invalid":
			return "A connection can only be narrowed, never widened. Pick a subset of what it already reaches.";
		case "forbidden":
			return "Connections are managed by their owner, signed in.";
		default:
			return "Check your connection and try again.";
	}
}

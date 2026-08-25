import type { WorkspaceAgent } from "./agents";

export type AgentRecord =
	| { kind: "loading" }
	| { kind: "ready"; value: WorkspaceAgent }
	| { kind: "missing" }
	| { kind: "forbidden" }
	| { kind: "authority_missing" }
	| { kind: "unavailable" };

export type AgentLifecycleAction = "rotate" | "disable" | "enable";

export type AgentLifecycleFailure =
	| { kind: "forbidden" }
	| { kind: "owner_invalid" }
	| { kind: "disabled" }
	| { kind: "active" }
	| { kind: "authority_missing" }
	| { kind: "unavailable" };

export function agentLifecycleFailureMessage(failure: AgentLifecycleFailure): string {
	switch (failure.kind) {
		case "forbidden":
			return "You may not manage this agent.";
		case "owner_invalid":
			return "This agent's owner is no longer an active person in this workspace. Register a replacement agent.";
		case "disabled":
			return "This agent is disabled. Enable it before issuing another credential.";
		case "active":
			return "This agent is already active.";
		case "authority_missing":
			return "The agent's previous authority could not be restored. Register a new agent instead.";
		case "unavailable":
			return "The agent could not be changed. Try again in a moment.";
	}
}

import type { components } from "$lib/api/dashboard.gen";
import type { WorkspaceAgent } from "$lib/agents/agents";
import { elapsed, onDateAndTime } from "$lib/time";

export type Runner = components["schemas"]["Runner"];
export type RunnerHost = components["schemas"]["RunnerHost"];
export type Codebase = components["schemas"]["Codebase"];
export type CodebaseState = components["schemas"]["CodebaseState"];
export type CodebaseRepository = components["schemas"]["CodebaseRepository"];
export type CodebaseRuntime = components["schemas"]["CodebaseRuntime"];
export type CodingTool = components["schemas"]["CodingTool"];
export type GatewayReach = components["schemas"]["GatewayReach"];
export type RemoteFingerprint = components["schemas"]["RemoteFingerprint"];

export type FolderReach = "ready" | "unreadable";

export type MachineAgent = {
	id: string;
	name: string;
	owner: string;
	disabled: boolean;
};

export type AgentMachines = {
	agent: MachineAgent;
	machines: Runner[];
	codebases: Codebase[];
	folders: FolderReach;
};

export type RunnersView =
	| { kind: "loading" }
	| { kind: "no_agents" }
	| { kind: "empty"; groups: AgentMachines[] }
	| { kind: "ready"; groups: AgentMachines[] }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

export function agentOf(registered: WorkspaceAgent): MachineAgent {
	return {
		id: registered.agent.id,
		name: registered.agent.name,
		owner: registered.ownerName || registered.ownerEmail || "somebody who has left",
		disabled: registered.agent.status === "disabled",
	};
}

export function agentOfMachine(machine: Runner): MachineAgent {
	return { id: machine.agentId, name: machine.agentName, owner: "", disabled: false };
}

export type RunnerFailure =
	| { kind: "gone" }
	| { kind: "forbidden" }
	| { kind: "already_disconnected" }
	| { kind: "unavailable" };

export const liveRefreshMs = 15_000;

export const installCommand = "curl -fsSL https://get.norn.so/runner | bash";

export const tokenPlaceholder = "<that agent's api token>";

export const reconfirmCommand = "norn runner inspect --confirm";

export function connectCommand(token: string): string {
	return `norn runner connect --token ${token}`;
}

export type MachineStanding =
	| "revoked"
	| "paused"
	| "never_connected"
	| "offline"
	| "disk_pressure"
	| "busy"
	| "ready";

export function machineStanding(machine: Runner): MachineStanding {
	if (machine.status === "revoked") return "revoked";
	if (machine.pausedAt) return "paused";
	if (!machine.lastSeenAt || !machine.load) return "never_connected";
	if (!machine.load.connected) return "offline";
	if (machine.load.diskPressure) return "disk_pressure";
	if (machine.load.free <= 0) return "busy";

	return "ready";
}

export const standingLabels: Record<MachineStanding, string> = {
	revoked: "Revoked",
	paused: "Paused",
	never_connected: "Never connected",
	offline: "Offline",
	disk_pressure: "Out of disk",
	busy: "At capacity",
	ready: "Taking work",
};

export type StandingTone = "waiting" | "working" | "attention" | "bad";

export const standingTones: Record<MachineStanding, StandingTone> = {
	revoked: "bad",
	paused: "waiting",
	never_connected: "waiting",
	offline: "waiting",
	disk_pressure: "attention",
	busy: "attention",
	ready: "working",
};

export function standingDetail(machine: Runner): string {
	switch (machineStanding(machine)) {
		case "revoked":
			return "This machine was cut off. Its agent still works everywhere else.";
		case "paused":
			return "Norn is not offering this machine new work. What it is already running carries on.";
		case "never_connected":
			return "This machine is bound to the agent but has never reported in. Start the runner on it.";
		case "offline":
			return "This machine is not holding a channel. It comes back on its own once the runner runs again.";
		case "disk_pressure":
			return "This machine is turning work down because it is short of disk.";
		case "busy":
			return "Every slot on this machine is in use. The next run waits for one to free.";
		case "ready":
			return "";
	}
}

export function slotLine(machine: Runner): string {
	if (!machine.load) return "No slots reported yet";

	return `${machine.load.used} of ${machine.load.capacity} slots in use`;
}

export function hostLine(host: RunnerHost): string {
	return [host.os, host.arch].filter(Boolean).join(" · ");
}

export function versionLine(host: RunnerHost): string {
	return host.version ? `Runner ${host.version}` : "Version unknown";
}

export function seenLine(machine: Runner, now: string, timezone: string): string {
	if (machine.status === "revoked") {
		return machine.revokedAt ? `Revoked ${onDateAndTime(machine.revokedAt, timezone)}` : "Revoked";
	}

	if (!machine.lastSeenAt) return `Enrolled ${onDateAndTime(machine.enrolledAt, timezone)}`;

	if (machine.load?.connected) return "Seen just now";

	return `Last seen ${elapsed(machine.lastSeenAt, now)} ago`;
}

export function repositoryPath(repository: CodebaseRepository): string {
	return repository.relPath === "." || repository.relPath === ""
		? "At the root of the folder"
		: repository.relPath;
}

export function remoteLabel(remote: RemoteFingerprint | undefined): string {
	if (!remote) return "";

	if (remote.host && remote.pathTail) return `${remote.host}/${remote.pathTail}`;

	return remote.pathTail || remote.host || "";
}

export const folderLabels: Record<CodebaseState, string> = {
	active: "Connected",
	drift: "Changed since you confirmed it",
	disconnected: "Disconnected",
};

export const folderTones: Record<CodebaseState, StandingTone> = {
	active: "working",
	drift: "attention",
	disconnected: "waiting",
};

export const runtimeLabels: Record<CodebaseRuntime, string> = {
	process: "Processes",
	docker: "Docker",
	kvm: "KVM",
};

export const gatewayLabels: Record<GatewayReach, string> = {
	reachable: "Previews reach this machine",
	unreachable: "Previews cannot reach this machine",
	unconfigured: "This instance serves no preview addresses",
};

export function toolLabel(tool: CodingTool): string {
	return tool.version ? `${tool.name} ${tool.version}` : tool.name;
}

export function machineFailure(error: { status?: number } | undefined): RunnerFailure {
	switch (error?.status) {
		case 403:
			return { kind: "forbidden" };
		case 404:
			return { kind: "gone" };
		case 409:
			return { kind: "already_disconnected" };
		default:
			return { kind: "unavailable" };
	}
}

export function failureMessage(failure: RunnerFailure): string {
	switch (failure.kind) {
		case "gone":
			return "That machine is no longer on record. Reload to see what is.";
		case "forbidden":
			return "You may not change this machine. Ask an administrator of this workspace.";
		case "already_disconnected":
			return "That folder was already disconnected.";
		case "unavailable":
			return "Something went wrong and nothing changed. Wait a moment and try again.";
	}
}

export function runnersPath(workspace: string): string {
	return `/${workspace}/settings/runners`;
}

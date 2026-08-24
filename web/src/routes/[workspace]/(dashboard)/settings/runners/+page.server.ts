import { keys } from "$lib/api/keys";
import type { WorkspaceAgent } from "$lib/agents/agents";
import {
	agentOf,
	agentOfMachine,
	type AgentMachines,
	type Codebase,
	type FolderReach,
	type MachineAgent,
	type Runner,
	type RunnersView,
} from "$lib/runners/runners";
import type { PageServerLoad } from "./$types";

export type RunnersPageData = { view: RunnersView; agents: WorkspaceAgent[] };

type Folders = { agentId: string; codebases: Codebase[]; reach: FolderReach };

function everyAgent(registered: WorkspaceAgent[], machines: Runner[]): MachineAgent[] {
	const known = registered.map(agentOf);
	const seen = new Set(known.map((agent) => agent.id));

	for (const machine of machines) {
		if (seen.has(machine.agentId)) continue;

		seen.add(machine.agentId);
		known.push(agentOfMachine(machine));
	}

	return known;
}

function grouped(agents: MachineAgent[], machines: Runner[], folders: Folders[]): AgentMachines[] {
	const held = new Map(folders.map((entry) => [entry.agentId, entry]));

	return agents
		.map((agent) => {
			const mine = machines.filter((machine) => machine.agentId === agent.id);
			const folder = held.get(agent.id);

			return {
				agent,
				machines: mine,
				codebases: folder?.codebases ?? [],
				folders: folder?.reach ?? "ready",
			} satisfies AgentMachines;
		})
		.sort((left, right) => right.machines.length - left.machines.length);
}

export const load: PageServerLoad = async ({
	depends,
	route,
	locals,
	parent,
}): Promise<RunnersPageData> => {
	depends(keys.page(route.id));

	const { workspace } = await parent();

	const [runners, agents] = await Promise.all([
		locals.api.GET("/workspaces/{workspaceId}/runners", {
			params: { path: { workspaceId: workspace.id } },
		}),
		locals.api.GET("/workspaces/{workspaceId}/agents", {
			params: { path: { workspaceId: workspace.id } },
		}),
	]);

	const registered = agents.data ?? [];

	if (runners.error) {
		return {
			view: { kind: runners.response.status === 403 ? "forbidden" : "unavailable" },
			agents: registered,
		};
	}

	const machines = runners.data ?? [];

	if (registered.length === 0 && machines.length === 0) {
		return { view: { kind: "no_agents" }, agents: registered };
	}

	const reachable = [...new Set(machines.map((machine) => machine.agentId))];

	const folders = await Promise.all(
		reachable.map(async (agentId): Promise<Folders> => {
			const codebases = await locals.api.GET(
				"/workspaces/{workspaceId}/agents/{agentId}/codebases",
				{ params: { path: { workspaceId: workspace.id, agentId } } }
			);

			if (codebases.error || !codebases.data) {
				return { agentId, codebases: [], reach: "unreadable" };
			}

			return { agentId, codebases: codebases.data, reach: "ready" };
		})
	);

	const groups = grouped(everyAgent(registered, machines), machines, folders);

	if (machines.length === 0) return { view: { kind: "empty", groups }, agents: registered };

	return { view: { kind: "ready", groups }, agents: registered };
};

import Folder from "@lucide/svelte/icons/folder";
import Layers from "@lucide/svelte/icons/layers";
import List from "@lucide/svelte/icons/list";
import Target from "@lucide/svelte/icons/target";
import UserRound from "@lucide/svelte/icons/user-round";
import Users from "@lucide/svelte/icons/users";
import { cyclePath, teamCyclesPath, type TeamCycle } from "$lib/cycles/cycles";
import { teamIssuesPath } from "$lib/issues/listing";
import { projectPath, projectsPath, teamProjectsPath, type Project } from "$lib/projects/projects";
import { resultPath, type SearchResult } from "$lib/search/search";
import { workspaceSettingsNavigation } from "$lib/settings/navigation";
import { destinations as chordDestinations } from "$lib/shortcuts/destinations";
import { displayKeys, shortcutOf } from "$lib/shortcuts/shortcuts";
import { teamPath, type Team } from "$lib/team/teams";
import { viewEntries, viewsPath, type SavedView } from "$lib/views/views";
import type { Membership } from "$lib/workspace/members";
import type { Destination } from "./model";

export type DestinationSources = {
	workspace: string;
	teams: Team[];
	cycles: TeamCycle[];
	views: SavedView[];
	projects: Project[];
	apple: boolean;
};

export function paletteDestinations(sources: DestinationSources): Destination[] {
	const { workspace, apple } = sources;

	const chords = chordDestinations(workspace).map(
		(destination): Destination => ({
			id: `nav:${destination.id}`,
			label: destination.label,
			href: destination.href,
			keys: displayKeys(shortcutOf(destination.id).keys[0], apple),
		})
	);

	const teams = sources.teams.flatMap((team): Destination[] => {
		const running = sources.cycles.find((entry) => entry.teamId === team.id);

		return [
			{ id: `team:${team.id}`, label: team.name, href: teamPath(workspace, team.key), context: "Team", icon: Users },
			{ id: `team-issues:${team.id}`, label: "Issues", href: teamIssuesPath(workspace, team.key), context: team.name, icon: List },
			{ id: `team-projects:${team.id}`, label: "Projects", href: teamProjectsPath(workspace, team.key), context: team.name, icon: Target },
			{ id: `team-cycles:${team.id}`, label: "Cycles", href: teamCyclesPath(workspace, team.key), context: team.name, icon: Layers },
			...(running
				? [
						{
							id: `cycle:${running.cycle.id}`,
							label: running.cycle.name,
							href: cyclePath(workspace, running.cycle),
							context: team.name,
							icon: Layers,
						},
					]
				: []),
		];
	});

	const entries = viewEntries(workspace, sources.views);
	const views = sources.views.map((view, index): Destination => ({
		id: `view:${view.id}`,
		label: view.name,
		href: entries[index].href,
		context: "View",
		icon: entries[index].icon,
	}));

	const projects = sources.projects.map((project): Destination => ({
		id: `project:${project.id}`,
		label: project.name,
		href: projectPath(workspace, project),
		context: "Project",
		icon: Folder,
	}));

	const settings = workspaceSettingsNavigation(workspace).flatMap((section) =>
		section.entries.map((entry): Destination => ({
			id: `settings:${entry.href}`,
			label: entry.label,
			href: entry.href,
			context: "Settings",
			icon: entry.icon,
		}))
	);

	return [
		...chords,
		{ id: "nav:projects", label: "Projects", href: projectsPath(workspace), icon: Target },
		{ id: "nav:views", label: "Views", href: viewsPath(workspace), icon: List },
		...teams,
		...views,
		...projects,
		...settings,
	];
}

export function peopleDestinations(workspace: string, members: Membership[]): Destination[] {
	return members.map((member) => {
		const name = member.displayName ?? member.email ?? "";

		return {
			id: `person:${member.accountId}`,
			label: name,
			href: resultPath(workspace, {
				kind: "person",
				id: member.accountId,
				title: name,
				titleHit: true,
				updatedAt: "",
			}),
			context: member.email,
			icon: UserRound,
		};
	});
}

export function resultDestinationId(result: SearchResult): string {
	switch (result.kind) {
		case "issue":
			return `issue:${result.id}`;
		case "comment":
			return `issue:${result.issueId ?? result.id}`;
		default:
			return `${result.kind}:${result.id}`;
	}
}

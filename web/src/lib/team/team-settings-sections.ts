import Bell from "@lucide/svelte/icons/bell";
import Bot from "@lucide/svelte/icons/bot";
import FileText from "@lucide/svelte/icons/file-text";
import GitBranch from "@lucide/svelte/icons/git-branch";
import Inbox from "@lucide/svelte/icons/inbox";
import Mail from "@lucide/svelte/icons/mail";
import RefreshCw from "@lucide/svelte/icons/refresh-cw";
import Settings from "@lucide/svelte/icons/settings";
import Users from "@lucide/svelte/icons/users";
import Workflow from "@lucide/svelte/icons/workflow";
import type { IconComponent } from "$lib/utils.js";
import { teamSettingsSections, type TeamSettingsSection } from "./teams";

export type TeamSettingsEntry = {
	section: TeamSettingsSection;
	title: string;
	description: string;
	icon: IconComponent;
};

export type TeamSettingsGroup = { label: string; entries: TeamSettingsEntry[] };

const entries: Record<TeamSettingsSection, TeamSettingsEntry> = {
	general: {
		section: "general",
		title: "General",
		description: "Name, icon, estimates, who can see the team, and archiving it.",
		icon: Settings,
	},
	members: {
		section: "members",
		title: "Members",
		description: "Who is on the team.",
		icon: Users,
	},
	notifications: {
		section: "notifications",
		title: "Notifications",
		description: "What you hear about from this team.",
		icon: Bell,
	},
	states: {
		section: "states",
		title: "States",
		description: "The steps an issue moves through, and where new issues start.",
		icon: Workflow,
	},
	templates: {
		section: "templates",
		title: "Issue templates",
		description: "The shapes issues are raised from.",
		icon: FileText,
	},
	cycles: {
		section: "cycles",
		title: "Cycles",
		description: "Whether work runs in cycles, how long each lasts and when it starts.",
		icon: RefreshCw,
	},
	triage: {
		section: "triage",
		title: "Triage",
		description: "Which new issues wait for somebody to accept them.",
		icon: Inbox,
	},
	email: {
		section: "email",
		title: "Issues by email",
		description: "An address that turns mail into issues for this team.",
		icon: Mail,
	},
	"source-control": {
		section: "source-control",
		title: "Source control",
		description: "Where linked changes move issues, and the branch name offered.",
		icon: GitBranch,
	},
	agents: {
		section: "agents",
		title: "Agents",
		description: "What an agent has to wait for a person to approve.",
		icon: Bot,
	},
};

export const teamSettingsGroups: TeamSettingsGroup[] = [
	{ label: "Team", entries: [entries.general, entries.members, entries.notifications] },
	{ label: "Work", entries: [entries.states, entries.templates, entries.cycles, entries.triage] },
	{
		label: "Intake & automation",
		entries: [entries.email, entries["source-control"], entries.agents],
	},
];

export function teamSettingsEntryAt(pathname: string): TeamSettingsEntry | null {
	const last = pathname.split("/").filter(Boolean).at(-1) ?? "";
	const section = teamSettingsSections.find((candidate) => candidate === last);

	return section ? entries[section] : null;
}

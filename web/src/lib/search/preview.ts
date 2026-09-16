import type {
	CommandRun,
	CommandScope,
	CommandStep,
	PaletteListing,
} from "$lib/command/model";

export type PalettePreview = {
	typed: string;
	step: CommandStep | null;
	scope: CommandScope;
	run: CommandRun;
	listing: PaletteListing;
};

export const palettePreviewStates: Record<string, PalettePreview> = import.meta.env.DEV
	? {
			recent: {
				typed: "",
				step: null,
				scope: {
					kind: "issues",
					issues: [{ id: "preview-mob-241", reference: "MOB-241", title: "Offline queue drops edits on reconnect", teamId: "preview-mobile" }],
				},
				run: { kind: "idle" },
				listing: {
					kind: "recent",
					groups: [
						{
							id: "recent",
							heading: "Recent",
							meta: "from this device",
							entries: [
								{ kind: "destination", destination: { id: "issue:preview-mob-241", label: "Offline queue drops edits on reconnect", href: "/northwind/issues/MOB-241", context: "MOB-241" } },
								{ kind: "destination", destination: { id: "issue:preview-bil-112", label: "Proration is off by one day on annual plans", href: "/northwind/issues/BIL-112", context: "BIL-112" } },
								{ kind: "destination", destination: { id: "view:preview-urgent", label: "Urgent & unassigned", href: "/northwind/issues?view=preview-urgent", context: "View" } },
							],
						},
						{
							id: "suggested",
							heading: "Suggested",
							meta: "commands",
							entries: [
								{ kind: "command", command: { id: "issue-new", label: "New issue", keys: "C" } },
								{ kind: "command", command: { id: "assign", label: "Assign MOB-241 to…", keys: "A", step: "assign" } },
								{ kind: "command", command: { id: "status", label: "Change status of MOB-241…", keys: "S", step: "status" } },
							],
						},
					],
				},
			},
			typing: {
				typed: "prorat",
				step: null,
				scope: { kind: "none" },
				run: { kind: "idle" },
				listing: {
					kind: "searching",
					groups: [
						{
							id: "places",
							heading: "Go to",
							entries: [
								{ kind: "destination", destination: { id: "project:preview-billing", label: "Billing hardening", href: "/northwind/projects/billing-hardening", context: "Project" } },
							],
						},
					],
				},
			},
			results: {
				typed: "board",
				step: null,
				scope: { kind: "none" },
				run: { kind: "idle" },
				listing: {
					kind: "results",
					fuzzy: false,
					groups: [
						{
							id: "issue",
							heading: "Issues",
							meta: "2",
							entries: [
								{ kind: "result", result: { kind: "issue", id: "preview-mob-236", title: "Keyboard navigation for the board view", reference: "MOB-236", titleHit: true, updatedAt: "2026-08-04T10:00:00Z" } },
								{ kind: "result", result: { kind: "issue", id: "preview-mob-233", title: "Board columns collapse state", reference: "MOB-233", titleHit: true, updatedAt: "2026-07-28T10:00:00Z" } },
							],
						},
						{
							id: "places",
							heading: "Go to",
							entries: [
								{ kind: "destination", destination: { id: "view:preview-board-bugs", label: "Board bugs", href: "/northwind/issues?view=preview-board-bugs", context: "View" } },
							],
						},
					],
				},
			},
			noresults: {
				typed: "gantt chart",
				step: null,
				scope: { kind: "none" },
				run: { kind: "idle" },
				listing: { kind: "no_matches", query: "gantt chart" },
			},
			slow: {
				typed: "invoice",
				step: null,
				scope: { kind: "none" },
				run: { kind: "idle" },
				listing: {
					kind: "slow",
					scanning: 48320,
					groups: [
						{
							id: "places",
							heading: "Go to",
							meta: "searching 48,320",
							entries: [
								{ kind: "destination", destination: { id: "issue:preview-bil-118", label: "Invoice PDFs should include the tax ID", href: "/northwind/issues/BIL-118", context: "BIL-118" } },
							],
						},
					],
				},
			},
			command: {
				typed: ">",
				step: null,
				scope: {
					kind: "issues",
					issues: [{ id: "preview-mob-241", reference: "MOB-241", title: "Offline queue drops edits on reconnect", teamId: "preview-mobile" }],
				},
				run: { kind: "idle" },
				listing: {
					kind: "commands",
					groups: [
						{
							id: "commands",
							heading: "Commands",
							meta: "8 available",
							entries: [
								{ kind: "command", command: { id: "issue-new", label: "New issue", keys: "C" } },
								{ kind: "command", command: { id: "assign", label: "Assign MOB-241 to…", keys: "A", step: "assign" } },
								{ kind: "command", command: { id: "status", label: "Change status of MOB-241…", keys: "S", step: "status" } },
								{ kind: "command", command: { id: "cycle", label: "Move MOB-241 to cycle…", step: "cycle" } },
								{ kind: "command", command: { id: "label", label: "Add label to MOB-241…", step: "label" } },
								{ kind: "command", command: { id: "copy-link", label: "Copy issue link" } },
								{ kind: "command", command: { id: "open-triage", label: "Open triage queue", keys: "G T" } },
								{ kind: "command", command: { id: "toggle-density", label: "Toggle compact density" } },
							],
						},
					],
				},
			},
			step2: {
				typed: "",
				step: { kind: "assign", label: "Assign MOB-241 to" },
				scope: {
					kind: "issues",
					issues: [{ id: "preview-mob-241", reference: "MOB-241", title: "Offline queue drops edits on reconnect", teamId: "preview-mobile" }],
				},
				run: { kind: "idle" },
				listing: {
					kind: "step",
					step: { kind: "assign", label: "Assign MOB-241 to" },
					groups: [
						{
							id: "step",
							heading: "Assign MOB-241 to",
							meta: "4 options",
							entries: [
								{ kind: "option", option: { value: "preview-rae", label: "Rae Okafor", hint: "rae@northwind.co", person: { accountId: "preview-rae", name: "Rae Okafor" } } },
								{ kind: "option", option: { value: "preview-jun", label: "Jun Park", hint: "jun@northwind.co", person: { accountId: "preview-jun", name: "Jun Park" } } },
								{ kind: "option", option: { value: "preview-milo", label: "Milo Vance", hint: "milo@northwind.co", person: { accountId: "preview-milo", name: "Milo Vance" } } },
								{ kind: "option", option: { value: "preview-ada", label: "Ada Ling", hint: "ada@northwind.co", person: { accountId: "preview-ada", name: "Ada Ling" } } },
							],
						},
					],
				},
			},
			executing: {
				typed: "",
				step: { kind: "assign", label: "Assign MOB-241 to" },
				scope: {
					kind: "issues",
					issues: [{ id: "preview-mob-241", reference: "MOB-241", title: "Offline queue drops edits on reconnect", teamId: "preview-mobile" }],
				},
				run: { kind: "running", entry: "preview-jun" },
				listing: {
					kind: "step",
					step: { kind: "assign", label: "Assign MOB-241 to" },
					groups: [
						{
							id: "step",
							heading: "Assign MOB-241 to",
							meta: "4 options",
							entries: [
								{ kind: "option", option: { value: "preview-rae", label: "Rae Okafor", hint: "rae@northwind.co", person: { accountId: "preview-rae", name: "Rae Okafor" } } },
								{ kind: "option", option: { value: "preview-jun", label: "Jun Park", hint: "jun@northwind.co", person: { accountId: "preview-jun", name: "Jun Park" } } },
								{ kind: "option", option: { value: "preview-milo", label: "Milo Vance", hint: "milo@northwind.co", person: { accountId: "preview-milo", name: "Milo Vance" } } },
								{ kind: "option", option: { value: "preview-ada", label: "Ada Ling", hint: "ada@northwind.co", person: { accountId: "preview-ada", name: "Ada Ling" } } },
							],
						},
					],
				},
			},
			failed: {
				typed: "",
				step: { kind: "assign", label: "Assign MOB-241 to" },
				scope: {
					kind: "issues",
					issues: [{ id: "preview-mob-241", reference: "MOB-241", title: "Offline queue drops edits on reconnect", teamId: "preview-mobile" }],
				},
				run: {
					kind: "failed",
					title: "Couldn’t assign MOB-241",
					detail: "MOB-241 · You cannot change this one",
				},
				listing: {
					kind: "step",
					step: { kind: "assign", label: "Assign MOB-241 to" },
					groups: [
						{
							id: "step",
							heading: "Assign MOB-241 to",
							meta: "4 options",
							entries: [
								{ kind: "option", option: { value: "preview-rae", label: "Rae Okafor", hint: "rae@northwind.co", person: { accountId: "preview-rae", name: "Rae Okafor" } } },
								{ kind: "option", option: { value: "preview-jun", label: "Jun Park", hint: "jun@northwind.co", person: { accountId: "preview-jun", name: "Jun Park" } } },
							],
						},
					],
				},
			},
			unavailable: {
				typed: "",
				step: null,
				scope: { kind: "none" },
				run: { kind: "idle" },
				listing: {
					kind: "unavailable",
					groups: [
						{
							id: "recent",
							heading: "Recent",
							meta: "from this device",
							entries: [
								{ kind: "destination", destination: { id: "issue:preview-mob-241", label: "Offline queue drops edits on reconnect", href: "/northwind/issues/MOB-241", context: "MOB-241" } },
								{ kind: "destination", destination: { id: "issue:preview-mob-236", label: "Keyboard navigation for the board view", href: "/northwind/issues/MOB-236", context: "MOB-236" } },
							],
						},
					],
				},
			},
		}
	: {};

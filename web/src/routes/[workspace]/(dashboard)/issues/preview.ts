import type { Task } from "$lib/tasks/types";

const bug = { name: "Bug", color: "magenta" } as const;
const design = { name: "Design", color: "violet" } as const;
const infra = { name: "Infra", color: "cyan" } as const;
const spec = { name: "Needs spec", color: "orchid" } as const;

function task(
	id: string,
	title: string,
	status: Task["status"],
	priority: Task["priority"],
	assignee: string | null,
	date: string | null,
	labels: Task["labels"],
	project: string
): Task {
	return {
		id,
		title,
		status,
		priority,
		assignee,
		date,
		labels,
		project,
		cycle: status === "backlog" ? null : "24",
	};
}

export const issuesFixture: Task[] = import.meta.env.DEV
	? [
			task("BIL-112", "Proration is off by one day on annual plans", "review", "urgent", "Rae Okafor", "Jul 28", [bug], "Billing"),
			task("MOB-241", "Offline queue drops edits on reconnect", "started", "urgent", "Rae Okafor", "Jul 29", [bug], "Mobile"),
			task("MOB-236", "Keyboard navigation for the board view", "review", "medium", "Rae Okafor", "Jul 30", [design], "Mobile"),
			task("BIL-118", "Invoice PDFs should include the tax ID", "todo", "high", "Rae Okafor", "Jul 30", [], "Billing"),
			task("MOB-238", "Ship the weekly digest email", "started", "high", "Rae Okafor", "Aug 1", [spec], "Mobile"),
			task("GRW-64", "Rewrite the empty states across settings", "todo", "medium", "Rae Okafor", "Aug 2", [design], "Growth"),
			task("MOB-244", "Cache avatars between sessions", "todo", "low", "Rae Okafor", "Aug 3", [infra], "Mobile"),
			task("GRW-71", "Instrument the onboarding funnel", "todo", "medium", "Rae Okafor", "Aug 8", [infra], "Growth"),
			task("BIL-121", "Dunning emails for failed cards", "backlog", "high", "Rae Okafor", null, [spec], "Billing"),
			task("MOB-249", "Drop support for iOS 15", "backlog", "low", "Rae Okafor", null, [], "Mobile"),
			task("MOB-252", "Realtime presence flickers on slow links", "started", "high", "Jun Park", "Aug 5", [bug], "Mobile"),
			task("GRW-73", "A/B test the invite email subject", "todo", "medium", "Jun Park", "Aug 6", [spec], "Growth"),
			task("BIL-124", "Support EU VAT on the team plan", "started", "medium", "Milo Vance", "Aug 8", [], "Billing"),
			task("MOB-247", "Swipe actions on the task row", "review", "medium", "Ada Ling", "Aug 4", [design], "Mobile"),
			task("GRW-75", "Referral landing page", "backlog", "low", null, null, [], "Growth"),
			task("MOB-230", "Audit contrast on the dusk theme", "done", "low", "Ada Ling", "Jul 29", [design], "Mobile"),
			task("GRW-58", "Landing page hero copy pass", "done", "medium", "Rae Okafor", "Jul 28", [], "Growth"),
			task("MOB-233", "Board columns collapse state", "done", "medium", "Jun Park", "Jul 27", [], "Mobile"),
			task("BIL-104", "Retire the legacy checkout", "canceled", "none", null, "Jul 22", [], "Billing"),
		]
	: [];

export const people = ["Rae Okafor", "Jun Park", "Milo Vance", "Ada Ling"];

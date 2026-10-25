import type { components } from "#lib/api/dashboard.gen.js";
import { onCalendarDay } from "#lib/time.js";

export type PublicRoadmap = components["schemas"]["PublicRoadmap"];
export type PublicRoadmapItem = components["schemas"]["PublicRoadmapItem"];
export type RoadmapPublicStatus = components["schemas"]["RoadmapPublicStatus"];
export type RoadmapTimeline = components["schemas"]["RoadmapTimeline"];
export type RoadmapStamp = "published" | "draft";

export type PublicRoadmapView =
	| { kind: "loading" }
	| { kind: "unavailable" }
	| { kind: "failed"; reference: string }
	| { kind: "empty"; roadmap: PublicRoadmap }
	| { kind: "ready"; roadmap: PublicRoadmap };

export type PublicRoadmapEntryView =
	| { kind: "loading" }
	| { kind: "unavailable" }
	| { kind: "failed"; reference: string }
	| { kind: "ready"; roadmapTitle: string; item: PublicRoadmapItem };

type Answer<T> = { data?: T; response: Response };

const notFound = 404;

function failure(answers: Answer<unknown>[], reference: string) {
	return answers.some(({ response }) => response.status === notFound)
		? ({ kind: "unavailable" } as const)
		: ({ kind: "failed", reference } as const);
}

export function readPublicRoadmap(answer: Answer<PublicRoadmap>, reference: string): PublicRoadmapView {
	const roadmap = answer.data;

	if (!answer.response.ok || !roadmap) return failure([answer], reference);

	if (roadmap.projects.length === 0 && roadmap.issues.length === 0) return { kind: "empty", roadmap };

	return { kind: "ready", roadmap };
}

export function readPublicRoadmapEntry(
	roadmapAnswer: Answer<PublicRoadmap>,
	itemAnswer: Answer<PublicRoadmapItem>,
	reference: string
): PublicRoadmapEntryView {
	const roadmap = roadmapAnswer.data;
	const item = itemAnswer.data;

	if (!roadmapAnswer.response.ok || !itemAnswer.response.ok || !roadmap || !item) {
		return failure([roadmapAnswer, itemAnswer], reference);
	}

	return { kind: "ready", roadmapTitle: roadmap.title, item };
}

export const statusLabels: Record<RoadmapPublicStatus, string> = {
	planned: "Planned",
	in_progress: "In progress",
	paused: "Paused",
	completed: "Completed",
	cancelled: "Cancelled",
};

export function timelineLabel(timeline: RoadmapTimeline | undefined): string | undefined {
	const start = timeline?.startOn;
	const end = timeline?.endOn;

	if (start && end) return `${onCalendarDay(start)} – ${onCalendarDay(end)}`;
	if (start) return `From ${onCalendarDay(start)}`;
	if (end) return `By ${onCalendarDay(end)}`;

	return undefined;
}

export function progressLabel(progress: number | undefined): string | undefined {
	return progress === undefined ? undefined : `${progress}% complete`;
}

export type TimelineWindow = { startOn: string; endOn: string };

export function timelineWindow(items: PublicRoadmapItem[]): TimelineWindow | undefined {
	const dates = items
		.flatMap(({ timeline }) => [timeline?.startOn, timeline?.endOn])
		.filter((date): date is string => date !== undefined)
		.sort();

	if (dates.length === 0) return undefined;

	return { startOn: dates[0], endOn: dates[dates.length - 1] };
}

export type TimelinePlacement =
	| { kind: "span"; offset: number; width: number }
	| { kind: "marker"; offset: number };

function dayOf(date: string): number {
	return Date.parse(`${date}T00:00:00Z`);
}

export function timelinePlacement(
	timeline: RoadmapTimeline | undefined,
	window: TimelineWindow
): TimelinePlacement | undefined {
	const first = dayOf(window.startOn);
	const length = dayOf(window.endOn) - first;
	const share = (date: string) => (length === 0 ? 0 : ((dayOf(date) - first) / length) * 100);

	const start = timeline?.startOn;
	const end = timeline?.endOn;

	if (start && end) return { kind: "span", offset: share(start), width: share(end) - share(start) };

	const only = start ?? end;

	return only ? { kind: "marker", offset: share(only) } : undefined;
}

export function issueCountLabel(item: PublicRoadmapItem): string | undefined {
	const count = item.issues?.length;

	if (count === undefined) return undefined;

	return count === 1 ? "1 issue" : `${count} issues`;
}

export function entryPath(id: string): string {
	return `/entries/${id}`;
}

export const roadmapPath = "/";

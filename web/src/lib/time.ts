const locale = "en-GB";

const dayMs = 86_400_000;

function parts(instant: string, timezone: string, options: Intl.DateTimeFormatOptions): string {
	return new Date(instant).toLocaleString(locale, { ...options, timeZone: timezone });
}

export function onDate(instant: string, timezone: string): string {
	return parts(instant, timezone, { year: "numeric", month: "short", day: "numeric" });
}

export function onDateAndTime(instant: string, timezone: string): string {
	return parts(instant, timezone, {
		month: "short",
		day: "numeric",
		hour: "2-digit",
		minute: "2-digit",
	});
}

export function atClock(instant: string, timezone: string): string {
	return parts(instant, timezone, { hour: "2-digit", minute: "2-digit", hour12: false });
}

export function onMonth(instant: string, timezone: string): string {
	return parts(instant, timezone, { year: "numeric", month: "short" });
}

export function daysBetween(from: string, to: string): number {
	return Math.floor((new Date(to).getTime() - new Date(from).getTime()) / dayMs);
}

export function joinedOn(instant: string | undefined, timezone: string): string {
	if (!instant) return "Joined";

	return `Joined ${onDate(instant, timezone)}`;
}

export function lastActive(instant: string | undefined, now: string, timezone: string): string {
	if (!instant) return "Never signed in";

	const days = daysBetween(instant, now);

	if (days <= 0) return "Active today";
	if (days === 1) return "Active yesterday";
	if (days < 30) return `Active ${days} days ago`;

	return `Active ${onMonth(instant, timezone)}`;
}

export function dueLabel(due: string | undefined, now: string, timezone: string): string {
	if (!due) return "No due date";

	const days = daysBetween(now, due);

	if (days < -1) return `Overdue by ${Math.abs(days)} days`;
	if (days === -1) return "Overdue by a day";
	if (days === 0) return "Due today";
	if (days === 1) return "Due tomorrow";
	if (days < 30) return `Due in ${days} days`;

    return `Due ${onDate(due, timezone)}`;
}

export function overdue(due: string | undefined, now: string): boolean {
	return Boolean(due) && daysBetween(now, due as string) < 0;
}

export function calendarDate(instant: string, timezone: string): string {
	const parts = new Intl.DateTimeFormat("en-CA", {
		year: "numeric",
		month: "2-digit",
		day: "2-digit",
		timeZone: timezone,
	}).format(new Date(instant));

	return parts;
}

export function shiftDays(date: string, days: number): string {
	const shifted = new Date(`${date}T00:00:00Z`).getTime() + days * dayMs;

	return new Date(shifted).toISOString().slice(0, 10);
}

export function onCalendarDate(date: string): string {
	return new Date(`${date}T00:00:00Z`).toLocaleString(locale, {
		month: "short",
		day: "numeric",
		timeZone: "UTC",
	});
}

export function cycleWindow(startsOn: string, endsOn: string): string {
	return `${onCalendarDate(startsOn)} – ${onCalendarDate(endsOn)}`;
}

import { formatBytes, type WorkspaceStorage } from "$lib/attachments/attachments";
import type { ProgressTone } from "$lib/components/ui/progress/progress.svelte";
import { onDateAndTime } from "$lib/time";

const nearlyFullPercent = 90;

export const headroomToneClass: Record<NonNullable<ProgressTone>, string> = {
	default: "text-muted-foreground",
	warning: "text-warning",
	destructive: "text-destructive",
};

export type StoragePressure = "comfortable" | "nearly_full" | "full";

export type StorageReading =
	| { kind: "unlimited"; storedBytes: number; measuredAt?: string }
	| {
			kind: "metered";
			storedBytes: number;
			maxBytes: number;
			remainingBytes: number;
			percent: number;
			pressure: StoragePressure;
			measuredAt?: string;
	  }
	| { kind: "unavailable" };

function pressureOf(percent: number, remainingBytes: number): StoragePressure {
	if (remainingBytes === 0) return "full";
	if (percent >= nearlyFullPercent) return "nearly_full";

	return "comfortable";
}

export function storageReading(storage: WorkspaceStorage | undefined): StorageReading {
	if (!storage) return { kind: "unavailable" };

	const storedBytes = Math.max(storage.storedBytes, 0);
	const measuredAt = storage.updatedAt;

	if (storage.unlimited || !storage.maxBytes || storage.maxBytes <= 0) {
		return { kind: "unlimited", storedBytes, measuredAt };
	}

	const maxBytes = storage.maxBytes;
	const remainingBytes = Math.max(maxBytes - storedBytes, 0);
	const percent = Math.min(Math.round((storedBytes / maxBytes) * 100), 100);

	return {
		kind: "metered",
		storedBytes,
		maxBytes,
		remainingBytes,
		percent,
		pressure: pressureOf(percent, remainingBytes),
		measuredAt,
	};
}

export function storageToneOf(reading: StorageReading): NonNullable<ProgressTone> {
	if (reading.kind !== "metered") return "default";

	switch (reading.pressure) {
		case "full":
			return "destructive";
		case "nearly_full":
			return "warning";
		default:
			return "default";
	}
}

export function storedLabel(reading: StorageReading): string {
	switch (reading.kind) {
		case "metered":
			return `${formatBytes(reading.storedBytes)} of ${formatBytes(reading.maxBytes)}`;
		case "unlimited":
			return formatBytes(reading.storedBytes);
		default:
			return "Unknown";
	}
}

export function headroomLabel(reading: StorageReading): string {
	if (reading.kind === "unlimited") return "This workspace has no storage limit.";
	if (reading.kind === "unavailable") return "";

	switch (reading.pressure) {
		case "full":
			return "This workspace is full. An upload will be refused until something is removed.";
		case "nearly_full":
			return `${formatBytes(reading.remainingBytes)} left. Uploads are refused once it runs out.`;
		default:
			return `${formatBytes(reading.remainingBytes)} left.`;
	}
}

export function measuredLabel(reading: StorageReading, timezone: string): string {
	if (reading.kind === "unavailable") return "";
	if (!reading.measuredAt) return "Norn has not measured this workspace yet.";

	return `Measured ${onDateAndTime(reading.measuredAt, timezone)}.`;
}

import { bindShortcuts } from "./registry.svelte";
import { registerRoam } from "./roam/roam.svelte";

export type ListCursorSurface<T> = {
	rows: readonly T[];
	open?: (row: T, index: number) => void;
	left?: () => boolean;
	right?: () => boolean;
};

export class ListCursor<T> {
	#surface: () => ListCursorSurface<T>;
	#wanted = $state(0);

	constructor(surface: () => ListCursorSurface<T>) {
		this.#surface = surface;

		$effect(() => {
			void this.at;

			document
				.querySelector('[data-cursor="true"]')
				?.scrollIntoView({ block: "nearest" });
		});
	}

	get #rows(): readonly T[] {
		return this.#surface().rows;
	}

	get at(): number {
		return Math.max(0, Math.min(this.#wanted, this.#rows.length - 1));
	}

	get row(): T | undefined {
		return this.#rows[this.at];
	}

	to(index: number) {
		this.#wanted = index;
	}

	move(by: number) {
		this.#wanted = Math.max(0, Math.min(this.at + by, this.#rows.length - 1));
	}

	open() {
		const row = this.row;

		if (row !== undefined) this.#surface().open?.(row, this.at);
	}

	holds(row: T): boolean {
		return row === this.row;
	}

	props(row: T) {
		const on = this.holds(row);

		return { "data-cursor": on, "aria-current": on ? ("true" as const) : undefined };
	}
}

export function listCursor<T>(surface: () => ListCursorSurface<T>): ListCursor<T> {
	const cursor = new ListCursor(surface);

	bindShortcuts({
		"cursor-down": () => cursor.move(1),
		"cursor-up": () => cursor.move(-1),
		"cursor-open": () => cursor.open(),
	});

	registerRoam(() => ({
		up: () => cursor.move(-1),
		down: () => cursor.move(1),
		left: () => surface().left?.() ?? false,
		right: () => surface().right?.() ?? false,
		enter: () => cursor.open(),
	}));

	return cursor;
}

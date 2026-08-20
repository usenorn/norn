import { untrack } from "svelte";
import { bindShortcuts } from "./registry.svelte";
import { registerRoam } from "./roam/roam.svelte";

export type ListCursorSurface<T> = {
	rows: readonly T[];
	open?: (row: T, index: number) => void;
	keyOf?: (row: T) => string;
	left?: () => boolean;
	right?: () => boolean;
};

export class ListCursor<T> {
	#surface: () => ListCursorSurface<T>;
	#wanted = $state(0);
	#held = $state.raw<string | undefined>(undefined);

	#index = $derived.by(() => {
		const rows = this.#rows;

		if (rows.length === 0) return 0;

		const held = this.#held;
		const found = held === undefined ? -1 : rows.findIndex((row) => this.#keyOf(row) === held);

		if (found >= 0) return found;

		return Math.max(0, Math.min(this.#wanted, rows.length - 1));
	});

	constructor(surface: () => ListCursorSurface<T>) {
		this.#surface = surface;

		$effect(() => {
			const rows = this.#rows;
			const wanted = this.#wanted;
			const held = untrack(() => this.#held);

			if (held !== undefined && rows.some((row) => this.#keyOf(row) === held)) return;

			const landed = rows[Math.max(0, Math.min(wanted, rows.length - 1))];

			this.#held = landed === undefined ? undefined : this.#keyOf(landed);
		});

		$effect(() => {
			void this.at;

			document.querySelector('[data-cursor="true"]')?.scrollIntoView({ block: "nearest" });
		});
	}

	get #rows(): readonly T[] {
		return this.#surface().rows;
	}

	#keyOf(row: T): string | undefined {
		const named = this.#surface().keyOf;

		if (named) return named(row);

		const id = (row as { id?: unknown }).id;

		return typeof id === "string" ? id : undefined;
	}

	get at(): number {
		return this.#index;
	}

	get row(): T | undefined {
		return this.#rows[this.at];
	}

	to(index: number) {
		const rows = this.#rows;
		const next = Math.max(0, Math.min(index, rows.length - 1));

		this.#wanted = next;
		this.#held = rows[next] === undefined ? undefined : this.#keyOf(rows[next]);
	}

	move(by: number) {
		this.to(this.at + by);
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

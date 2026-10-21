import { tick, untrack } from "svelte";
import { afterNavigate } from "$app/navigation";
import { bindShortcuts } from "./registry.svelte";
import { registerRoam } from "./roam/roam.svelte";

export type ListCursorSurface<T> = {
	rows: readonly T[];
	open?: (row: T, index: number) => void;
	keyOf?: (row: T) => string;
	referenceOf?: (row: T) => string;
	left?: () => boolean;
	right?: () => boolean;
};

export class ListCursor<T> {
	#surface: () => ListCursorSurface<T>;
	#wanted = $state(0);
	#held = $state.raw<string | undefined>(undefined);
	#following = false;

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

			if (!this.#following) {
				this.#following = true;

				return;
			}

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

	land(reference: string): boolean {
		const referenceOf = this.#surface().referenceOf;

		if (!referenceOf) return false;

		const index = this.#rows.findIndex((row) => referenceOf(row) === reference);

		if (index < 0) return false;

		this.to(index);

		return true;
	}

	async reveal() {
		await tick();

		const anchor = document.querySelector<HTMLElement>(
			'[data-cursor="true"][data-row-anchor], [data-cursor="true"] [data-row-anchor]'
		);

		anchor?.focus({ preventScroll: true });
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

	const step = (by: number) => {
		cursor.move(by);
		void cursor.reveal();
	};

	bindShortcuts({
		"cursor-down": () => step(1),
		"cursor-up": () => step(-1),
		"cursor-open": () => cursor.open(),
	});

	afterNavigate(({ from }) => {
		const reference = from?.params?.reference;

		if (!reference || !from?.route.id?.endsWith("/issues/[reference]")) return;
		if (cursor.land(reference)) void cursor.reveal();
	});

	registerRoam(() => ({
		up: () => step(-1),
		down: () => step(1),
		left: () => surface().left?.() ?? false,
		right: () => surface().right?.() ?? false,
		enter: () => cursor.open(),
	}));

	return cursor;
}

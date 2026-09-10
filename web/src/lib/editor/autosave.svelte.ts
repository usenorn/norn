export type SaveOutcome = "saved" | "conflict" | "failed" | "unknown";

export type SaveState =
	| { kind: "idle" }
	| { kind: "waiting" }
	| { kind: "saving" }
	| { kind: "saved" }
	| { kind: "conflict" }
	| { kind: "failed" }
	| { kind: "unknown" };

export const saveDelay = 1200;

/**
 * Autosave keeps one save in the air at a time. Two saves racing is how a description ends up
 * holding whichever answer came back last rather than what was typed last, and a save that is
 * skipped because one was already running is how the final sentence never lands.
 *
 * Typing is never blocked: a save runs beside it, and anything typed while it runs is saved
 * straight afterwards.
 */
export class Autosave {
	#state = $state.raw<SaveState>({ kind: "idle" });
	#timer: ReturnType<typeof setTimeout> | null = null;
	#running = false;
	#again = false;
	#save: () => Promise<SaveOutcome>;

	constructor(save: () => Promise<SaveOutcome>) {
		this.#save = save;
	}

	get state(): SaveState {
		return this.#state;
	}

	get settled(): boolean {
		return !this.#running && this.#timer === null;
	}

	schedule() {
		this.#clear();
		this.#state = { kind: "waiting" };
		this.#timer = setTimeout(() => {
			this.#timer = null;
			void this.#run();
		}, saveDelay);
	}

	async flush(): Promise<SaveOutcome | null> {
		this.#clear();

		return this.#run();
	}

	// A refusal stops the clock. Retrying on a timer against a description somebody else has
	// moved on would keep failing silently; the person is told and decides.
	rest() {
		this.#clear();
		this.#state = { kind: "idle" };
	}

	stop() {
		this.#clear();
	}

	#clear() {
		if (this.#timer === null) return;

		clearTimeout(this.#timer);
		this.#timer = null;
	}

	async #run(): Promise<SaveOutcome | null> {
		if (this.#running) {
			this.#again = true;

			return null;
		}

		this.#running = true;
		this.#state = { kind: "saving" };

		let outcome: SaveOutcome;

		try {
			outcome = await this.#save();
		} catch {
			outcome = "unknown";
		} finally {
			this.#running = false;
		}

		this.#state = { kind: outcome === "saved" ? "saved" : outcome };

		if (outcome === "saved" && this.#again) {
			this.#again = false;

			return this.#run();
		}

		this.#again = false;

		return outcome;
	}
}

export function saveLine(state: SaveState): string {
	switch (state.kind) {
		case "waiting":
			return "Saving shortly";
		case "saving":
			return "Saving";
		case "saved":
			return "Saved";
		case "conflict":
			return "Somebody else saved first";
		case "failed":
			return "Not saved";
		case "unknown":
			return "We could not tell whether this saved";
		default:
			return "";
	}
}

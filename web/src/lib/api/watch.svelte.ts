import type { ApiResult } from "./attempt";

export type Watched<T> =
	| { kind: "idle" }
	| { kind: "watching"; value?: T }
	| { kind: "unreadable"; value?: T };

export type Watching<T> = {
	read: (id: string) => Promise<ApiResult<T>>;
	settled: (value: T) => boolean;
	identify: (value: T) => string;
	every?: number;
	patience?: number;
};

export class Watch<T> {
	#state = $state.raw<Watched<T>>({ kind: "idle" });
	#watching: Watching<T>;
	#timer: ReturnType<typeof setTimeout> | null = null;
	#id = "";
	#missed = 0;

	constructor(watching: Watching<T>) {
		this.#watching = watching;
	}

	get state(): Watched<T> {
		return this.#state;
	}

	get value(): T | undefined {
		return this.#state.kind === "idle" ? undefined : this.#state.value;
	}

	start(id: string, seed?: T) {
		this.#clear();
		this.#id = id;
		this.#missed = 0;
		this.#state = { kind: "watching", value: seed };

		this.#later();
	}

	stop() {
		this.#clear();
		this.#id = "";
		this.#state = { kind: "idle" };
	}

	retry() {
		if (this.#id === "") return;

		this.#missed = 0;
		this.#state = { kind: "watching", value: this.value };

		void this.#once();
	}

	#clear() {
		if (this.#timer === null) return;

		clearTimeout(this.#timer);
		this.#timer = null;
	}

	#later() {
		this.#clear();
		this.#timer = setTimeout(() => {
			this.#timer = null;
			void this.#once();
		}, this.#watching.every ?? 700);
	}

	async #once() {
		const mine = this.#id;

		if (mine === "") return;

		let result: ApiResult<T>;

		try {
			result = await this.#watching.read(mine);
		} catch {
			this.#missing(mine);

			return;
		}

		if (this.#id !== mine) return;

		if (result.error !== undefined || result.data === undefined) {
			this.#missing(mine);

			return;
		}

		if (this.#watching.identify(result.data) !== mine) return;

		this.#missed = 0;
		this.#state = { kind: "watching", value: result.data };

		if (this.#watching.settled(result.data)) {
			this.#clear();
			this.#id = "";

			return;
		}

		this.#later();
	}

	#missing(mine: string) {
		if (this.#id !== mine) return;

		this.#missed += 1;

		if (this.#missed > (this.#watching.patience ?? 4)) {
			this.#clear();
			this.#state = { kind: "unreadable", value: this.value };

			return;
		}

		this.#clear();
		this.#timer = setTimeout(() => {
			this.#timer = null;
			void this.#once();
		}, (this.#watching.every ?? 700) * this.#missed);
	}
}

export class Pending {
	#held = $state.raw<string[]>([]);

	busy(key: string): boolean {
		return this.#held.includes(key);
	}

	get any(): boolean {
		return this.#held.length > 0;
	}

	async once<T>(key: string, run: () => Promise<T>): Promise<T | undefined> {
		if (this.busy(key)) return undefined;

		this.#held = [...this.#held, key];

		try {
			return await run();
		} finally {
			this.#held = this.#held.filter((held) => held !== key);
		}
	}
}

import { describe, expect, it, vi } from "vitest";
import { Autosave, saveLine } from "./autosave.svelte";

function deferred<T>() {
	let settle: (value: T) => void = () => {};
	const promise = new Promise<T>((resolve) => (settle = resolve));

	return { promise, settle };
}

describe("Autosave", () => {
	it("runs one save at a time and saves again for what was typed while it ran", async () => {
		const first = deferred<"saved">();
		const second = deferred<"saved">();
		const saves = [first, second];
		let started = 0;

		const autosave = new Autosave(() => {
			const held = saves[started];

			started += 1;

			return held.promise;
		});

		const running = autosave.flush();

		expect(started).toBe(1);

		void autosave.flush();

		expect(started).toBe(1);

		first.settle("saved");
		second.settle("saved");

		await running;

		expect(started).toBe(2);
	});

	it("says it is saving while a save is in the air and saved once it lands", async () => {
		const held = deferred<"saved">();
		const autosave = new Autosave(() => held.promise);

		const running = autosave.flush();

		expect(autosave.state.kind).toBe("saving");

		held.settle("saved");
		await running;

		expect(autosave.state.kind).toBe("saved");
	});

	it("reports a refusal rather than claiming the writing is safe", async () => {
		const autosave = new Autosave(async () => "conflict" as const);

		await autosave.flush();

		expect(autosave.state.kind).toBe("conflict");
		expect(saveLine(autosave.state)).toBe("Somebody else saved first");
	});

	it("treats a save that threw as an outcome nobody knows", async () => {
		const autosave = new Autosave(async () => {
			throw new Error("the network went away");
		});

		await autosave.flush();

		expect(autosave.state.kind).toBe("unknown");
	});

	it("waits before saving and saves once when typing stops", async () => {
		vi.useFakeTimers();

		let saved = 0;
		const autosave = new Autosave(async () => {
			saved += 1;

			return "saved" as const;
		});

		autosave.schedule();
		autosave.schedule();
		autosave.schedule();

		expect(autosave.state.kind).toBe("waiting");
		expect(saved).toBe(0);

		await vi.runAllTimersAsync();

		expect(saved).toBe(1);

		vi.useRealTimers();
	});
});

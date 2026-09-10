import { describe, expect, it, vi } from "vitest";
import { Watch } from "./watch.svelte";

type Run = { id: string; status: string };

function watching(answers: (() => Promise<{ data?: Run; error?: unknown }>)[]) {
	let at = 0;

	return new Watch<Run>({
		read: () => answers[Math.min(at++, answers.length - 1)](),
		settled: (value) => value.status === "complete",
		identify: (value) => value.id,
		every: 10,
	});
}

describe("Watch", () => {
	it("stops once the operation settles", async () => {
		vi.useFakeTimers();

		const watch = watching([
			async () => ({ data: { id: "one", status: "running" } }),
			async () => ({ data: { id: "one", status: "complete" } }),
		]);

		watch.start("one");
		await vi.advanceTimersByTimeAsync(100);

		expect(watch.state.kind).toBe("watching");
		expect(watch.value?.status).toBe("complete");

		vi.useRealTimers();
	});

	it("asks for nothing when the seed has already settled", async () => {
		vi.useFakeTimers();

		let asked = 0;

		const watch = new Watch<Run>({
			read: async () => {
				asked += 1;

				return { data: { id: "one", status: "complete" } };
			},
			settled: (value) => value.status === "complete",
			identify: (value) => value.id,
			every: 10,
		});

		watch.start("one", { id: "one", status: "complete" });
		await vi.advanceTimersByTimeAsync(100);

		expect(asked).toBe(0);
		expect(watch.value?.status).toBe("complete");

		vi.useRealTimers();
	});

	it("keeps trying a readable failure rather than stopping silently", async () => {
		vi.useFakeTimers();

		let asked = 0;

		const watch = new Watch<Run>({
			read: async () => {
				asked += 1;

				return asked < 3
					? { error: { status: 503 } }
					: { data: { id: "one", status: "complete" } };
			},
			settled: (value) => value.status === "complete",
			identify: (value) => value.id,
			every: 10,
		});

		watch.start("one", { id: "one", status: "running" });
		await vi.advanceTimersByTimeAsync(200);

		expect(asked).toBeGreaterThanOrEqual(3);
		expect(watch.value?.status).toBe("complete");

		vi.useRealTimers();
	});

	it("says it cannot read the status rather than showing it as running forever", async () => {
		vi.useFakeTimers();

		const watch = new Watch<Run>({
			read: async () => ({ error: { status: 500 } }),
			settled: (value) => value.status === "complete",
			identify: (value) => value.id,
			every: 10,
			patience: 2,
		});

		watch.start("one", { id: "one", status: "running" });
		await vi.advanceTimersByTimeAsync(500);

		expect(watch.state.kind).toBe("unreadable");
		expect(watch.value?.status).toBe("running");

		vi.useRealTimers();
	});

	it("throws away a late answer belonging to an operation that was replaced", async () => {
		vi.useFakeTimers();

		const watch = new Watch<Run>({
			read: async (id) => ({ data: { id: id === "one" ? "one" : "two", status: "running" } }),
			settled: (value) => value.status === "complete",
			identify: (value) => value.id,
			every: 10,
		});

		watch.start("one");
		watch.start("two");

		await vi.advanceTimersByTimeAsync(50);

		expect(watch.value?.id).toBe("two");

		vi.useRealTimers();
	});

	it("stops when the surface goes away", async () => {
		vi.useFakeTimers();

		let asked = 0;

		const watch = new Watch<Run>({
			read: async () => {
				asked += 1;

				return { data: { id: "one", status: "running" } };
			},
			settled: (value) => value.status === "complete",
			identify: (value) => value.id,
			every: 10,
		});

		watch.start("one");
		await vi.advanceTimersByTimeAsync(50);

		const seen = asked;

		watch.stop();
		await vi.advanceTimersByTimeAsync(200);

		expect(asked).toBe(seen);
		expect(watch.state.kind).toBe("idle");

		vi.useRealTimers();
	});
});

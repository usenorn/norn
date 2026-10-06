import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const held = vi.hoisted(() => ({
	calls: [] as string[],
	settle: [] as (() => void)[],
	route: { to: null as unknown },
}));

vi.mock("$app/navigation", () => ({
	invalidate: (key: string) =>
		new Promise<void>((done) => {
			held.calls.push(key);
			held.settle.push(done);
		}),
	invalidateAll: () =>
		new Promise<void>((done) => {
			held.calls.push("*");
			held.settle.push(done);
		}),
}));

vi.mock("$app/state", () => ({ navigating: held.route }));

import { RealtimeConnection, refetchWindowMs } from "./connection.svelte";

function land() {
	for (const done of held.settle.splice(0)) done();
}

describe("refetching after a realtime event", () => {
	beforeEach(() => {
		vi.useFakeTimers();
		held.calls.length = 0;
		held.settle.length = 0;
		held.route.to = null;
	});

	afterEach(() => {
		vi.useRealTimers();
	});

	it("batches the keys and invalidates them once the window closes", async () => {
		const realtime = new RealtimeConnection();

		realtime.refetch("issues");
		realtime.refetch("issues", "triage");

		expect(held.calls).toEqual([]);

		await vi.advanceTimersByTimeAsync(refetchWindowMs);

		expect(held.calls).toEqual(["issues", "triage"]);
	});

	it("waits for a navigation that is under way instead of cancelling it, then applies the keys", async () => {
		const realtime = new RealtimeConnection();

		held.route.to = { url: new URL("https://norn.test/w/issues/BIL-6") };
		realtime.refetch("issues");

		await vi.advanceTimersByTimeAsync(refetchWindowMs);
		await vi.advanceTimersByTimeAsync(refetchWindowMs);

		expect(held.calls).toEqual([]);

		held.route.to = null;

		await vi.advanceTimersByTimeAsync(refetchWindowMs);

		expect(held.calls).toEqual(["issues"]);
	});

	it("keeps keys that arrive while an invalidate is still running and applies them next", async () => {
		const realtime = new RealtimeConnection();

		realtime.refetch("issues");
		await vi.advanceTimersByTimeAsync(refetchWindowMs);

		expect(held.calls).toEqual(["issues"]);

		realtime.refetch("inbox");
		land();
		await vi.advanceTimersByTimeAsync(refetchWindowMs);

		expect(held.calls).toEqual(["issues", "inbox"]);
	});

	it("turns a resync into one invalidateAll that also waits for the navigation", async () => {
		const realtime = new RealtimeConnection();

		held.route.to = { url: new URL("https://norn.test/w/issues/BIL-6") };
		realtime.refetch("issues");
		realtime.resync();

		await vi.advanceTimersByTimeAsync(refetchWindowMs);

		expect(held.calls).toEqual([]);

		held.route.to = null;

		await vi.advanceTimersByTimeAsync(refetchWindowMs);

		expect(held.calls).toEqual(["*"]);

		land();
		await vi.advanceTimersByTimeAsync(refetchWindowMs);

		expect(held.calls).toEqual(["*"]);
	});

	it("drops what it was holding when the connection closes", async () => {
		const realtime = new RealtimeConnection();

		realtime.refetch("issues");
		realtime.resync();
		realtime.close();

		await vi.advanceTimersByTimeAsync(refetchWindowMs * 2);

		expect(held.calls).toEqual([]);
		expect(vi.getTimerCount()).toBe(0);
	});
});

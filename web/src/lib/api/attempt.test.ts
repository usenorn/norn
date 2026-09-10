import { describe, expect, it } from "vitest";
import { attempt, outcomeLine } from "./attempt";
import { Pending } from "./pending.svelte";

describe("attempt", () => {
	it("hands back what the server returned when it answered", async () => {
		const outcome = await attempt({ run: async () => ({ data: { id: "one" } }) });

		expect(outcome).toEqual({ kind: "done", value: { id: "one" } });
	});

	it("calls a refusal a refusal and puts the optimistic paint back", async () => {
		let painted = false;

		const outcome = await attempt({
			run: async () => ({ error: { code: "issue_stale" }, response: { status: 409 } as Response }),
			optimistic: () => (painted = true),
			reconcile: () => (painted = false),
		});

		expect(outcome.kind).toBe("refused");
		expect(outcome.kind === "refused" && outcome.status).toBe(409);
		expect(painted).toBe(false);
	});

	it("calls a server error unknown, because the write may still have landed", async () => {
		const outcome = await attempt({
			run: async () => ({ error: { title: "Internal" }, response: { status: 500 } as Response }),
		});

		expect(outcome.kind).toBe("unknown");
	});

	it("calls a thrown request unknown rather than refused", async () => {
		let painted = false;

		const outcome = await attempt({
			run: async () => {
				throw new TypeError("Failed to fetch");
			},
			optimistic: () => (painted = true),
			reconcile: () => (painted = false),
		});

		expect(outcome.kind).toBe("unknown");
		expect(painted).toBe(false);
	});

	it("treats an answer carrying neither data nor an error as unknown", async () => {
		const outcome = await attempt({ run: async () => ({}) });

		expect(outcome.kind).toBe("unknown");
	});

	it("says the outcome is unknown rather than borrowing the refusal's words", () => {
		const refused = outcomeLine(
			{ kind: "refused", problem: null, status: 403 },
			"That was not allowed."
		);
		const unknown = outcomeLine({ kind: "unknown" }, "That was not allowed.");

		expect(refused).toBe("That was not allowed.");
		expect(unknown).not.toBe(refused);
		expect(unknown).toContain("could not tell");
	});
});

describe("Pending", () => {
	it("refuses to run the same operation twice at once", async () => {
		const pending = new Pending();
		let started = 0;

		const held = pending.once("issue-1", async () => {
			started += 1;

			await new Promise((settle) => setTimeout(settle, 5));

			return "first";
		});

		const second = await pending.once("issue-1", async () => {
			started += 1;

			return "second";
		});

		expect(second).toBeUndefined();
		expect(await held).toBe("first");
		expect(started).toBe(1);
	});

	it("lets a different operation through while one is pending", async () => {
		const pending = new Pending();

		const held = pending.once("issue-1", () => new Promise((settle) => setTimeout(settle, 5)));

		expect(pending.busy("issue-1")).toBe(true);
		expect(pending.busy("issue-2")).toBe(false);

		await held;

		expect(pending.busy("issue-1")).toBe(false);
	});

	it("frees the operation even when it threw", async () => {
		const pending = new Pending();

		await expect(
			pending.once("issue-1", async () => {
				throw new Error("no");
			})
		).rejects.toThrow();

		expect(pending.busy("issue-1")).toBe(false);
	});
});

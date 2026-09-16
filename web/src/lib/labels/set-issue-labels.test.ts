import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Issue } from "$lib/issues/issues";
import { forget } from "$lib/issues/versions";

const put = vi.hoisted(() => vi.fn());

vi.mock("$lib/api", () => ({ api: { PUT: put } }));

const { setIssueLabels } = await import("./set-issue-labels");

type Answer = { data?: unknown; error?: unknown; response: { status: number } };

function held() {
	let answer: (value: Answer) => void = () => {};
	const promise = new Promise<Answer>((settle) => (answer = settle));

	return { promise, answer };
}

const issue = { id: "issue-one", version: 3, labels: [] } as unknown as Issue;

function answered(version: number, labelIds: string[]): Answer {
	return {
		data: { ...issue, version, labels: labelIds.map((id) => ({ id, name: id })) },
		response: { status: 200 },
	};
}

function sent(call: number): { expectedVersion: number; labelIds: string[] } {
	return put.mock.calls[call][1].body;
}

describe("picking labels one after another on an issue", () => {
	beforeEach(() => {
		forget();
		put.mockReset();
	});

	it("waits for the first answer and claims the version it returned", async () => {
		const first = held();
		const second = held();

		put.mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise);

		const adding = setIssueLabels({ workspaceId: "w", issue, labelIds: ["bug"] });
		const addingMore = setIssueLabels({ workspaceId: "w", issue, labelIds: ["bug", "ui"] });

		await Promise.resolve();
		expect(put).toHaveBeenCalledTimes(1);

		first.answer(answered(4, ["bug"]));
		expect((await adding).kind).toBe("changed");

		await vi.waitFor(() => expect(put).toHaveBeenCalledTimes(2));
		expect(sent(1)).toEqual({ expectedVersion: 4, labelIds: ["bug", "ui"] });

		second.answer(answered(5, ["bug", "ui"]));
		const outcome = await addingMore;

		expect(outcome.kind === "changed" && outcome.issue.labels.map((label) => label.id)).toEqual([
			"bug",
			"ui",
		]);
		expect(put).toHaveBeenCalledTimes(2);
	});

	it("sends only the latest set when several picks wait behind one request", async () => {
		const first = held();

		put.mockReturnValueOnce(first.promise).mockResolvedValueOnce(answered(5, ["bug", "ui", "api"]));

		setIssueLabels({ workspaceId: "w", issue, labelIds: ["bug"] });
		const skipped = setIssueLabels({ workspaceId: "w", issue, labelIds: ["bug", "ui"] });
		const latest = setIssueLabels({ workspaceId: "w", issue, labelIds: ["bug", "ui", "api"] });

		expect((await skipped).kind).toBe("superseded");

		first.answer(answered(4, ["bug"]));

		expect((await latest).kind).toBe("changed");
		expect(put).toHaveBeenCalledTimes(2);
		expect(sent(1)).toEqual({ expectedVersion: 4, labelIds: ["bug", "ui", "api"] });
	});

	it("puts the optimistic change back when the server refuses it", async () => {
		const reconcile = vi.fn();

		put.mockResolvedValueOnce({
			error: { code: "issue_stale", status: 409 },
			response: { status: 409 },
		});

		const outcome = await setIssueLabels({
			workspaceId: "w",
			issue,
			labelIds: ["bug"],
			reconcile,
		});

		expect(outcome.kind).toBe("refused");
		expect(reconcile).toHaveBeenCalledTimes(1);
	});
});

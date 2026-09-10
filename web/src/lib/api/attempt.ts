export type ApiResult<T> = { data?: T; error?: unknown; response?: Response };

export type Outcome<T> =
	| { kind: "done"; value: T }
	| { kind: "refused"; problem: unknown; status: number }
	| { kind: "unknown" };

export type Attempt<T> = {
	run: () => Promise<ApiResult<T>>;
	optimistic?: () => void;
	reconcile?: () => void;
};

export async function attempt<T>(asked: Attempt<T>): Promise<Outcome<T>> {
	asked.optimistic?.();

	let result: ApiResult<T>;

	try {
		result = await asked.run();
	} catch {
		asked.reconcile?.();

		return { kind: "unknown" };
	}

	if (result.data !== undefined && result.error === undefined) {
		return { kind: "done", value: result.data };
	}

	asked.reconcile?.();

	const status = result.response?.status ?? 0;

	if (decided(status)) return { kind: "refused", problem: result.error, status };

	return { kind: "unknown" };
}

export function decided(status: number): boolean {
	return status >= 400 && status < 500;
}

export const unknownLine =
	"We could not tell whether that went through. Nothing was lost — reload to see where it stands.";

export function outcomeLine(outcome: Outcome<unknown>, refused: string): string {
	return outcome.kind === "unknown" ? unknownLine : refused;
}

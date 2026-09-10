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

	if (answered(result)) return { kind: "done", value: result.data as T };

	asked.reconcile?.();

	const status = result.response?.status ?? 0;

	if (decided(status)) return { kind: "refused", problem: result.error, status };

	return { kind: "unknown" };
}

function answered(result: ApiResult<unknown>): boolean {
	if (result.error !== undefined) return false;

	const status = result.response?.status;

	return status === undefined ? result.data !== undefined : status < 300;
}

export function decided(status: number): boolean {
	return status >= 400 && status < 500;
}

export const unknownLine =
	"We could not tell whether that went through. Nothing was lost — reload to see where it stands.";

export function outcomeLine(outcome: Outcome<unknown>, refused: string): string {
	return outcome.kind === "unknown" ? unknownLine : refused;
}

import type { components } from "$lib/api/dashboard.gen";

export type TeamIntakeAddress = components["schemas"]["TeamIntakeAddress"];

export type IntakeSetting =
	| { kind: "loading" }
	| { kind: "unavailable" }
	| { kind: "unconfigured" }
	| { kind: "off" }
	| { kind: "on"; address: TeamIntakeAddress };

export type IntakeFailure =
	| { kind: "unconfigured" }
	| { kind: "forbidden" }
	| { kind: "taken" }
	| { kind: "unavailable" };

const failureMessages: Record<IntakeFailure["kind"], string> = {
	unconfigured: "This instance has no domain for incoming mail yet. An administrator sets one.",
	forbidden: "You cannot change this team's settings.",
	taken: "That address was taken while we were writing it. Try again.",
	unavailable: "Nothing changed. Wait a moment and try again.",
};

export function intakeFailureMessage(failure: IntakeFailure): string {
	return failureMessages[failure.kind];
}

export function readIntakeFailure(problem: unknown): IntakeFailure {
	if (typeof problem !== "object" || problem === null) return { kind: "unavailable" };

	if ("status" in problem && problem.status === 403) return { kind: "forbidden" };

	if ("code" in problem && problem.code === "intake_domain_unset") {
		return { kind: "unconfigured" };
	}

	if ("code" in problem && problem.code === "intake_address_taken") {
		return { kind: "taken" };
	}

	return { kind: "unavailable" };
}

export function intakeFor(
	address: TeamIntakeAddress | undefined,
	status: number
): IntakeSetting {
	if (address) return { kind: "on", address };
	if (status === 404) return { kind: "off" };

	return { kind: "unavailable" };
}

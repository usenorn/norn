import type { components } from "$lib/api/dashboard.gen";

export type IssueQuestion = components["schemas"]["IssueQuestion"];

export function statusLine(question: IssueQuestion): string {
	switch (question.state) {
		case "answered":
			return "Answered";
		case "dismissed":
			return question.blocking ? "Dismissed, and the run stopped" : "Dismissed";
		case "expired":
			return question.blocking ? "Nobody answered, so the run stopped" : "Working on the default";
		default:
			break;
	}

	if (question.expired) {
		return question.blocking ? "Out of time" : "Working on the default";
	}

	return question.blocking ? "The run is waiting on you" : "Waiting on you";
}

export function standingLine(question: IssueQuestion): string {
	if (question.answered) {
		return `${question.answeredByName ?? "Someone"} answered: ${question.answer}`;
	}

	if (question.state === "dismissed") {
		return `${question.settledByName ?? "Someone"} decided this was not going to be answered.`;
	}

	if (question.blocking) {
		return "The agent stopped here and cannot go on until somebody decides.";
	}

	return `Working on the default meanwhile: ${question.default}`;
}

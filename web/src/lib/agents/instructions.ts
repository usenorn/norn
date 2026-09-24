export const agentInstructionsMaxLength = 20000;

export const agentInstructionsTooLong = `Keep the instructions under ${agentInstructionsMaxLength} characters.`;

export function agentInstructionsMessage(code: string): string {
	switch (code) {
		case "too_long":
			return agentInstructionsTooLong;
		case "malformed":
			return "Remove the control characters and try again.";
		default:
			return "Those instructions cannot be saved.";
	}
}

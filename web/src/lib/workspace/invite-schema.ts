import { z } from "zod";
import { isEmailAddress, parseAddresses } from "./invites";

export const inviteSchema = z.object({
	addresses: z
		.string()
		.trim()
		.min(1, "Paste at least one email address.")
		.refine((text) => parseAddresses(text).some(isEmailAddress), {
			error: "None of those look like email addresses.",
		}),
});

export type InviteInput = z.infer<typeof inviteSchema>;

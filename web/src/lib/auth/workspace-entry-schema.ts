import { z } from "zod";

export const workspaceEntrySchema = z.object({
	workspace: z
		.string()
		.trim()
		.min(1, "Enter the workspace address you sign in at.")
		.max(64, "That is longer than any workspace address."),
});

export type WorkspaceEntryInput = z.infer<typeof workspaceEntrySchema>;

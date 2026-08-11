import { z } from "zod";

export const delegateIssueSchema = z.object({
	agentAccountId: z.string().uuid("Choose which agent takes this on"),
	brief: z.string().max(4000, "That brief is longer than Norn will carry").default(""),
});

export type DelegateIssueInput = z.infer<typeof delegateIssueSchema>;

import { z } from "zod";
import { decisionChannels } from "./decision-channel";

export const decisionChannelSchema = z.object({
	channel: z.enum(decisionChannels),
});

export type DecisionChannelInput = z.infer<typeof decisionChannelSchema>;

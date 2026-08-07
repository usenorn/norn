import { z } from "zod";

export const samlConnectionSchema = z
	.object({
		source: z.enum(["url", "paste", "manual"]).default("url"),
		metadataUrl: z.string().trim().default(""),
		metadata: z.string().trim().default(""),
		entityId: z.string().trim().default(""),
		ssoUrl: z.string().trim().default(""),
		certificate: z.string().trim().default(""),
		emailAttribute: z.string().trim().default(""),
		nameAttribute: z.string().trim().default(""),
		groupsAttribute: z.string().trim().default(""),
		allowIdpInitiated: z.boolean().default(false),
		adminGroup: z.string().trim().default(""),
		provisioning: z.boolean().default(false),
	})
	.superRefine((value, ctx) => {
		const require = (path: "metadataUrl" | "metadata" | "entityId" | "ssoUrl" | "certificate", message: string) => {
			if (!value[path]) ctx.addIssue({ code: "custom", path: [path], message });
		};

		if (value.source === "url") {
			require("metadataUrl", "Enter the metadata address your provider publishes.");
		}

		if (value.source === "paste") {
			require("metadata", "Paste the metadata document your provider gave you.");
		}

		if (value.source === "manual") {
			require("entityId", "Enter the provider's entity ID.");
			require("ssoUrl", "Enter the provider's sign-in URL.");
			require("certificate", "Paste the provider's signing certificate.");
		}
	});

export type SamlConnectionInput = z.infer<typeof samlConnectionSchema>;

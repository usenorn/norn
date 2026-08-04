import { z } from "zod";

const providerUrl = z
	.string()
	.trim()
	.refine(
		(value) => {
			try {
				const parsed = new URL(value);

				if (parsed.protocol === "https:") return true;

				return (
					parsed.protocol === "http:" &&
					["localhost", "127.0.0.1", "[::1]", "host.docker.internal"].includes(parsed.host.split(":")[0])
				);
			} catch {
				return false;
			}
		},
		"Enter a full https address, for example https://login.example.com."
	);

export const ssoConnectionSchema = z
	.object({
		issuer: providerUrl,
		manual: z.boolean().default(false),
		authorizationEndpoint: z.string().trim().default(""),
		tokenEndpoint: z.string().trim().default(""),
		jwksUri: z.string().trim().default(""),
		userinfoEndpoint: z.string().trim().default(""),
		clientId: z.string().trim().min(1, "Enter the client ID your provider issued."),
		clientSecret: z.string().default(""),
		scopes: z.string().trim().default("openid email profile"),
		groupsClaim: z.string().trim().default(""),
		provisioning: z.boolean().default(false),
	})
	.superRefine((value, ctx) => {
		if (!value.manual) return;

		for (const field of ["authorizationEndpoint", "tokenEndpoint", "jwksUri"] as const) {
			const result = providerUrl.safeParse(value[field]);

			if (!result.success) {
				ctx.addIssue({
					code: "custom",
					path: [field],
					message: result.error.issues[0].message,
				});
			}
		}
	});

export type SsoConnectionInput = z.infer<typeof ssoConnectionSchema>;

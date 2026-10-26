import { defineEnvVars } from "@sveltejs/kit/env";

const optional = (value: string | undefined) => value;

export const variables = defineEnvVars({
	PUBLIC_ROADMAPS_DOMAIN: { public: true, schema: optional },
	INTERNAL_API_ORIGIN: { schema: optional },
});

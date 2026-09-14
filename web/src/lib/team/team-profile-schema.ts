import { teamSettingsSchema } from "$lib/team/team-settings-schema";

export const teamProfileSchema = teamSettingsSchema.pick({ name: true, description: true });

export type TeamProfileInput = typeof teamProfileSchema._output;

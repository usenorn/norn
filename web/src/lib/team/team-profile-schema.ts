import { teamSettingsSchema } from "$lib/team/team-settings-schema";

export const teamProfileSchema = teamSettingsSchema.pick({ name: true, description: true });

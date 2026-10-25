import { teamSettingsSchema } from "#lib/team/team-settings-schema.js";

export const teamProfileSchema = teamSettingsSchema.pick({ name: true, description: true });

import { describe, expect, it } from "vitest";
import { avatarToneOf, avatarTones } from "./avatar-tone";

function accountIds(count: number): string[] {
	return Array.from(
		{ length: count },
		(_, index) => `0f6c2a4e-3b1d-4c8a-9e7f-${index.toString(16).padStart(12, "0")}`
	);
}

describe("the colour behind a person's initials", () => {
	it("is the same every time for the same account", () => {
		const accountId = "92196aec-bf4c-4e5b-b268-65699ba72659";

		expect(avatarToneOf(accountId)).toBe(avatarToneOf(accountId));
		expect(avatarToneOf(accountId.toUpperCase())).toBe(avatarToneOf(accountId));
	});

	it("comes from the palette", () => {
		for (const accountId of accountIds(50)) {
			expect(avatarTones).toContain(avatarToneOf(accountId));
		}
	});

	it("spreads different accounts across the whole palette", () => {
		const seen = new Map<string, number>();
		const total = 800;

		for (const accountId of accountIds(total)) {
			const tone = avatarToneOf(accountId);

			seen.set(tone, (seen.get(tone) ?? 0) + 1);
		}

		const fair = total / avatarTones.length;

		expect(seen.size).toBe(avatarTones.length);

		for (const count of seen.values()) {
			expect(count).toBeGreaterThan(fair / 2);
			expect(count).toBeLessThan(fair * 2);
		}
	});
});

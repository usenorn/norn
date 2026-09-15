import { flushSync, mount, unmount } from "svelte";
import { afterEach, describe, expect, it } from "vitest";
import PersonAvatar from "./person-avatar.svelte";

const mounted: { target: HTMLElement; held: ReturnType<typeof mount> }[] = [];

function shown(props: { accountId: string; name: string; avatarUrl?: string }) {
	const target = document.createElement("div");

	document.body.append(target);

	const held = mount(PersonAvatar, { target, props });

	flushSync();
	mounted.push({ target, held });

	return target;
}

afterEach(() => {
	for (const { target, held } of mounted.splice(0)) {
		void unmount(held);
		target.remove();
	}
});

function toneIn(target: HTMLElement): string | null {
	return target.querySelector("[data-slot='avatar']")?.getAttribute("data-tone") ?? null;
}

describe("a person's avatar", () => {
	const accountId = "ac447a37-d755-447d-b37f-6aea8679d9c8";

	it("keeps its colour when the person is renamed, and only the initials change", () => {
		const before = shown({ accountId, name: "Vlad Gorokhov" });
		const after = shown({ accountId, name: "Vladislav Petrov" });

		expect(toneIn(before)).not.toBeNull();
		expect(toneIn(after)).toBe(toneIn(before));
		expect(before.textContent).toContain("VG");
		expect(after.textContent).toContain("VP");
	});

	it("still shows a real picture when the account has one", () => {
		const target = shown({ accountId, name: "Vlad Gorokhov", avatarUrl: "https://norn.test/vlad.png" });

		expect(target.querySelector("img")?.getAttribute("src")).toBe("https://norn.test/vlad.png");
	});
});

import { describe, expect, it } from "vitest";
import { pastedFiles } from "./clipboard";

const at = new Date("2026-09-11T14:32:05");

type Shape = {
	items?: { kind: string; file: File | null }[];
	files?: File[];
	types?: string[];
};

function clipboard(shape: Shape): DataTransfer {
	const items = (shape.items ?? []).map((held) => ({
		kind: held.kind,
		type: held.file?.type ?? "",
		getAsFile: () => held.file,
	}));

	return {
		items,
		files: shape.files ?? [],
		types: shape.types ?? [],
	} as unknown as DataTransfer;
}

function shot(name: string, type = "image/png", bytes = 4): File {
	return new File([new Uint8Array(bytes)], name, { type });
}

describe("reading files out of a paste", () => {
	it("finds a screenshot that only arrives as an item, which is what Safari sends", () => {
		const found = pastedFiles(
			clipboard({ items: [{ kind: "file", file: shot("image.png") }], types: ["Files"] }),
			at
		);

		expect(found).toHaveLength(1);
	});

	it("counts a screenshot once when the clipboard carries it twice", () => {
		const held = shot("shot.png");

		const found = pastedFiles(
			clipboard({ items: [{ kind: "file", file: held }], files: [held], types: ["Files"] }),
			at
		);

		expect(found).toHaveLength(1);
	});

	it("leaves rich text alone, because its pictures already have an address", () => {
		const found = pastedFiles(
			clipboard({
				items: [{ kind: "file", file: shot("from-page.png") }],
				types: ["text/html", "text/plain", "Files"],
			}),
			at
		);

		expect(found).toEqual([]);
	});

	it("takes nothing from plain text", () => {
		expect(pastedFiles(clipboard({ types: ["text/plain"] }), at)).toEqual([]);
		expect(pastedFiles(null, at)).toEqual([]);
	});

	it("ignores an empty file", () => {
		const found = pastedFiles(
			clipboard({ items: [{ kind: "file", file: shot("empty.png", "image/png", 0) }] }),
			at
		);

		expect(found).toEqual([]);
	});

	it("names a screenshot that arrives without one, which the server would otherwise refuse", () => {
		const [found] = pastedFiles(clipboard({ items: [{ kind: "file", file: shot("") }] }), at);

		expect(found.name).toBe("screenshot-2026-09-11-143205.png");
	});

	it("names the one every browser calls image.png, so a workspace is not full of them", () => {
		const [png] = pastedFiles(clipboard({ items: [{ kind: "file", file: shot("image.png") }] }), at);
		const [jpeg] = pastedFiles(
			clipboard({ items: [{ kind: "file", file: shot("image.jpeg", "image/jpeg") }] }),
			at
		);

		expect(png.name).toBe("screenshot-2026-09-11-143205.png");
		expect(jpeg.name).toBe("screenshot-2026-09-11-143205.jpg");
	});

	it("keeps a name somebody chose", () => {
		const [found] = pastedFiles(
			clipboard({ items: [{ kind: "file", file: shot("the-bug.png") }] }),
			at
		);

		expect(found.name).toBe("the-bug.png");
	});
});

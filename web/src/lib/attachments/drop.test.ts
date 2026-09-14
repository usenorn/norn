import { describe, expect, it, vi } from "vitest";
import { dropZone } from "./drop";

function drag(types: string[], files: File[] = []) {
	const event = {
		dataTransfer: { types, files },
		defaultPrevented: false,
		preventDefault() {
			(this as { defaultPrevented: boolean }).defaultPrevented = true;
		},
	};

	return event as unknown as DragEvent;
}

function zone(accepts = true) {
	const took = vi.fn();
	const shown = vi.fn();

	return {
		took,
		shown,
		lit: () => shown.mock.calls.at(-1)?.[0] ?? false,
		handlers: dropZone({ take: took, over: shown, accepts: () => accepts }),
	};
}

const held = [new File([new Uint8Array(4)], "shot.png", { type: "image/png" })];
const files = () => drag(["Files"], held);

describe("a drop zone that takes files", () => {
	it("stays lit while the cursor crosses its own children, which is what made it blink", () => {
		const { handlers, lit } = zone();

		handlers.ondragenter(files());
		handlers.ondragenter(files());
		handlers.ondragleave(files());

		expect(lit()).toBe(true);
	});

	it("goes dark when the cursor has left everything inside it", () => {
		const { handlers, lit } = zone();

		handlers.ondragenter(files());
		handlers.ondragenter(files());
		handlers.ondragleave(files());
		handlers.ondragleave(files());

		expect(lit()).toBe(false);
	});

	it("lights up again after a drag that already ended", () => {
		const { handlers, lit } = zone();

		handlers.ondragenter(files());
		handlers.ondrop(files());
		handlers.ondragenter(files());

		expect(lit()).toBe(true);
	});

	it("ignores a drag carrying no file, so dragging text inside it changes nothing", () => {
		const { handlers, shown, took } = zone();

		handlers.ondragenter(drag(["text/plain"]));
		handlers.ondrop(drag(["text/plain"]));

		expect(shown).not.toHaveBeenCalled();
		expect(took).not.toHaveBeenCalled();
	});

	it("takes the files of a drop nobody handled", () => {
		const { handlers, took } = zone();

		handlers.ondrop(files());

		expect(took).toHaveBeenCalledWith(held);
	});

	it("leaves a drop the editor already handled alone, which is what attached it twice", () => {
		const { handlers, took, lit } = zone();
		const event = files();

		event.preventDefault();
		handlers.ondrop(event);

		expect(took).not.toHaveBeenCalled();
		expect(lit()).toBe(false);
	});

	it("neither lights up nor takes anything while the zone is busy", () => {
		const { handlers, took, lit } = zone(false);

		handlers.ondragenter(files());
		handlers.ondrop(files());

		expect(lit()).toBe(false);
		expect(took).not.toHaveBeenCalled();
	});

	it("lets the browser drop on it at all, which needs the default prevented while over it", () => {
		const { handlers } = zone();
		const event = files();

		handlers.ondragover(event);

		expect(event.defaultPrevented).toBe(true);
	});
});

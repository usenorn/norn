import type { Attachment } from "svelte/attachments";

const holdDelay = 400;
const moveTolerance = 8;
const scrollEdge = 48;
const scrollStep = 14;

export type TouchDragHandlers = {
	start: () => boolean;
	move: (x: number, y: number) => void;
	drop: () => void;
	cancel: () => void;
};

export function touchDrag(handlers: TouchDragHandlers | undefined): Attachment<HTMLElement> {
	return (node) => {
		if (!handlers) return;

		let timer: ReturnType<typeof setTimeout> | undefined;
		let pointer: number | undefined;
		let originX = 0;
		let originY = 0;
		let active = false;
		let swallowClick = false;

		const wait = () => {
			clearTimeout(timer);
			timer = undefined;
		};

		const finish = (dropped: boolean) => {
			wait();
			pointer = undefined;

			if (!active) return;

			active = false;
			swallowClick = true;
			delete node.dataset.touchDragging;

			if (dropped) handlers.drop();
			else handlers.cancel();
		};

		const down = (event: PointerEvent) => {
			if (event.pointerType !== "touch" || !event.isPrimary) return;

			pointer = event.pointerId;
			originX = event.clientX;
			originY = event.clientY;
			swallowClick = false;

			timer = setTimeout(() => {
				timer = undefined;

				if (!handlers.start()) return;

				active = true;
				node.dataset.touchDragging = "true";
				navigator.vibrate?.(10);
			}, holdDelay);
		};

		const move = (event: PointerEvent) => {
			if (event.pointerId !== pointer) return;

			if (timer) {
				const drifted =
					Math.abs(event.clientX - originX) > moveTolerance ||
					Math.abs(event.clientY - originY) > moveTolerance;

				if (drifted) wait();

				return;
			}

			if (active) handlers.move(event.clientX, event.clientY);
		};

		const up = (event: PointerEvent) => {
			if (event.pointerId === pointer) finish(true);
		};

		const cancel = (event: PointerEvent) => {
			if (event.pointerId === pointer) finish(false);
		};

		const holdScroll = (event: TouchEvent) => {
			if (active) event.preventDefault();
		};

		const holdMenu = (event: Event) => {
			if (active || timer) event.preventDefault();
		};

		const holdClick = (event: MouseEvent) => {
			if (!swallowClick) return;

			swallowClick = false;
			event.preventDefault();
			event.stopPropagation();
		};

		node.addEventListener("pointerdown", down);
		node.addEventListener("pointermove", move);
		node.addEventListener("pointerup", up);
		node.addEventListener("pointercancel", cancel);
		node.addEventListener("touchmove", holdScroll, { passive: false });
		node.addEventListener("contextmenu", holdMenu);
		node.addEventListener("click", holdClick, true);

		return () => {
			wait();
			node.removeEventListener("pointerdown", down);
			node.removeEventListener("pointermove", move);
			node.removeEventListener("pointerup", up);
			node.removeEventListener("pointercancel", cancel);
			node.removeEventListener("touchmove", holdScroll);
			node.removeEventListener("contextmenu", holdMenu);
			node.removeEventListener("click", holdClick, true);
		};
	};
}

export function edgeScroll(scroller: HTMLElement | undefined, x: number, y: number): number {
	if (!scroller) return 0;

	const box = scroller.getBoundingClientRect();
	const dx = x < box.left + scrollEdge ? -scrollStep : x > box.right - scrollEdge ? scrollStep : 0;
	const dy = y < box.top + scrollEdge ? -scrollStep : y > box.bottom - scrollEdge ? scrollStep : 0;

	if (dx !== 0 || dy !== 0) scroller.scrollBy(dx, dy);

	return Math.abs(dx) + Math.abs(dy);
}

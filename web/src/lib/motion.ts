import type { AnimationConfig } from "svelte/animate";
import type { TransitionConfig } from "svelte/transition";

export const DURATION = {
	none: 0,
	hover: 70,
	fast: 110,
	base: 160,
	slow: 240,
	panel: 320,
} as const;

export const EASING = {
	out: [0.22, 0.61, 0.36, 1],
	inOut: [0.4, 0, 0.2, 1],
	entrance: [0.16, 1, 0.3, 1],
	exit: [0.4, 0, 1, 1],
} as const;

function cubicBezier(p1x: number, p1y: number, p2x: number, p2y: number): (x: number) => number {
	const a = (one: number, two: number) => 1 - 3 * two + 3 * one;
	const b = (one: number, two: number) => 3 * two - 6 * one;
	const c = (one: number) => 3 * one;
	const at = (t: number, one: number, two: number) =>
		((a(one, two) * t + b(one, two)) * t + c(one)) * t;
	const slope = (t: number, one: number, two: number) =>
		3 * a(one, two) * t * t + 2 * b(one, two) * t + c(one);

	return (x) => {
		if (x <= 0) return 0;
		if (x >= 1) return 1;

		let t = x;

		for (let pass = 0; pass < 4; pass++) {
			const gradient = slope(t, p1x, p2x);

			if (gradient === 0) break;

			t -= (at(t, p1x, p2x) - x) / gradient;
		}

		return at(t, p1y, p2y);
	};
}

export const ease = {
	out: cubicBezier(...EASING.out),
	inOut: cubicBezier(...EASING.inOut),
	entrance: cubicBezier(...EASING.entrance),
	exit: cubicBezier(...EASING.exit),
};

function reduced(): boolean {
	return (
		typeof window !== "undefined" &&
		typeof window.matchMedia === "function" &&
		window.matchMedia("(prefers-reduced-motion: reduce)").matches
	);
}

type Composite = { duration: number; easing: (t: number) => number; css: (t: number) => string };

function composite({ duration, easing, css }: Composite) {
	return (_node: Element, params?: Partial<Composite>): TransitionConfig => {
		if (reduced()) return { duration: 0 };

		return { duration, easing, css, ...params };
	};
}

export const pop = composite({
	duration: DURATION.base,
	easing: ease.entrance,
	css: (t) => `opacity:${t};transform:translateY(${(1 - t) * -3}px)`,
});

export const lift = composite({
	duration: DURATION.base,
	easing: ease.entrance,
	css: (t) => `opacity:${t};transform:translateY(${(1 - t) * 6}px)`,
});

export const mark = composite({
	duration: DURATION.fast,
	easing: ease.out,
	css: (t) => `opacity:${t}`,
});

export const scrim = composite({
	duration: DURATION.fast,
	easing: ease.out,
	css: (t) => `opacity:${t}`,
});

export type Side = "top" | "right" | "bottom" | "left";

export function sheet(
	_node: Element,
	{ side = "right", duration = DURATION.panel }: { side?: Side; duration?: number } = {}
): TransitionConfig {
	if (reduced()) return { duration: 0 };

	const axis = side === "left" || side === "right" ? "X" : "Y";
	const sign = side === "left" || side === "top" ? -1 : 1;

	return {
		duration,
		easing: ease.entrance,
		css: (t) => `transform:translate${axis}(${(1 - t) * 100 * sign}%)`,
	};
}

export function slot(
	_node: Element,
	{ from, to }: { from: DOMRect; to: DOMRect },
	{ duration = DURATION.base }: { duration?: number } = {}
): AnimationConfig {
	if (reduced()) return { duration: 0 };

	const dx = from.left - to.left;
	const dy = from.top - to.top;

	return {
		duration,
		easing: ease.out,
		css: (_t, u) => `transform:translate(${u * dx}px, ${u * dy}px)`,
	};
}

export function flash(node: Element | null | undefined, duration = DURATION.slow): void {
	if (!node || reduced()) return;

	node.classList.remove("animate-flash");
	void (node as HTMLElement).offsetWidth;
	node.classList.add("animate-flash");
	setTimeout(() => node.classList.remove("animate-flash"), duration + 20);
}

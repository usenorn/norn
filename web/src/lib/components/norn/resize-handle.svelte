<script lang="ts">
	import { cn } from "$lib/utils.js";

	let {
		side,
		width,
		minimum,
		maximum,
		label,
		onresize,
		oncollapse,
		class: className,
	}: {
		side: "leading" | "trailing";
		width: number;
		minimum: number;
		maximum: number;
		label: string;
		onresize: (width: number, room: number) => void;
		oncollapse: () => void;
		class?: string;
	} = $props();

	const step = 16;

	let dragging = $state(false);
	let origin = 0;
	let started = 0;

	function travelled(clientX: number): number {
		const distance = clientX - origin;

		return side === "leading" ? started + distance : started - distance;
	}

	function grab(event: PointerEvent) {
		if (event.button !== 0) return;

		event.preventDefault();
		dragging = true;
		origin = event.clientX;
		started = width;
	}

	function nudge(event: KeyboardEvent) {
		const room = window.innerWidth;
		const toward = side === "leading" ? step : -step;

		switch (event.key) {
			case "ArrowLeft":
				onresize(width - toward, room);
				break;
			case "ArrowRight":
				onresize(width + toward, room);
				break;
			case "Home":
				onresize(side === "leading" ? minimum : maximum, room);
				break;
			case "End":
				onresize(side === "leading" ? maximum : minimum, room);
				break;
			case "Enter":
			case " ":
				oncollapse();
				break;
			default:
				return;
		}

		event.preventDefault();
	}

	$effect(() => {
		if (!dragging) return;

		const drag = (event: PointerEvent) => onresize(travelled(event.clientX), window.innerWidth);
		const release = () => (dragging = false);

		window.addEventListener("pointermove", drag);
		window.addEventListener("pointerup", release);
		window.addEventListener("pointercancel", release);
		document.body.classList.add("select-none", "cursor-col-resize");

		return () => {
			window.removeEventListener("pointermove", drag);
			window.removeEventListener("pointerup", release);
			window.removeEventListener("pointercancel", release);
			document.body.classList.remove("select-none", "cursor-col-resize");
		};
	});
</script>

<!-- A focusable separator is the W3C window-splitter pattern; svelte-check models
	separator as never interactive, so both rules misfire on it. -->
<!-- svelte-ignore a11y_no_noninteractive_tabindex -->
<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<div
	role="separator"
	aria-orientation="vertical"
	aria-label={label}
	aria-valuenow={width}
	aria-valuemin={minimum}
	aria-valuemax={maximum}
	tabindex="0"
	onpointerdown={grab}
	ondblclick={oncollapse}
	onkeydown={nudge}
	class={cn(
		"group absolute inset-y-0 z-10 flex w-2.5 cursor-col-resize touch-none items-stretch justify-center focus-visible:outline-none",
		side === "leading" ? "right-0 translate-x-1/2" : "left-0 -translate-x-1/2",
		className
	)}
>
	<span
		class={cn(
			"w-0.5 rounded-full bg-transparent motion-control group-hover:bg-line-strong group-focus-visible:bg-ring",
			dragging && "bg-ring"
		)}
	></span>
</div>

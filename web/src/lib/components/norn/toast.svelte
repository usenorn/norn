<script lang="ts" module>
	import { tv, type VariantProps } from "tailwind-variants";

	export const toastVariants = tv({
		base: "notch-ink flex max-w-full items-center gap-2.5 px-3 py-2 text-md",
		variants: {
			tone: {
				default: "[--keycap-lip:var(--ink-700)]",
				failure:
					"[--notch-ink-surface:var(--destructive)] [--notch-ink-fg:var(--destructive-foreground)] [--keycap-lip:var(--red-600)]",
			},
		},
		defaultVariants: { tone: "default" },
	});

	export type ToastTone = VariantProps<typeof toastVariants>["tone"];
</script>

<script lang="ts">
	import { cn } from "$lib/utils.js";

	let {
		message,
		href,
		detail,
		action = "Undo",
		tone = "default",
		onaction,
		onnavigate,
		class: className,
	}: {
		message: string;
		href?: string;
		detail?: string;
		action?: string;
		tone?: ToastTone;
		onaction?: () => void;
		onnavigate?: () => void;
		class?: string;
	} = $props();
</script>

<div
	role={tone === "failure" ? "alert" : "status"}
	aria-live={tone === "failure" ? "assertive" : "polite"}
	aria-atomic="true"
	class={cn(toastVariants({ tone }), className)}
>
	<div class="min-w-0 flex-1">
		{#if href}
			<a {href} onclick={onnavigate} class="block underline-offset-2 text-pretty hover:underline">
				{message}
			</a>
		{:else}
			<span class="block text-pretty">{message}</span>
		{/if}
		{#if detail}
			<p class="mt-0.5 line-clamp-2 text-xs opacity-75">{detail}</p>
		{/if}
	</div>
	{#if onaction}
		<button
			type="button"
			onclick={onaction}
			class="motion-control shrink-0 cursor-pointer border-b border-transparent px-0.5 font-mono text-2xs font-medium tracking-caps uppercase hover:border-primary-foreground"
		>
			{action}
		</button>
	{/if}
</div>

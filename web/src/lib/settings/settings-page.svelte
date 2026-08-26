<script module lang="ts">
	import { tv, type VariantProps } from "tailwind-variants";

	export const settingsPageContent = tv({
		base: "mx-auto flex w-full flex-col gap-5 px-4 py-5 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))] sm:px-5 sm:py-6",
		variants: {
			width: {
				compact: "max-w-160",
				standard: "max-w-224",
				wide: "max-w-256",
			},
		},
		defaultVariants: {
			width: "standard",
		},
	});

	export type SettingsPageWidth = VariantProps<typeof settingsPageContent>["width"];
</script>

<script lang="ts">
	import type { Component, Snippet } from "svelte";

	let {
		title,
		description,
		Icon,
		meta,
		width = "standard",
		actions,
		children,
	}: {
		title: string;
		description: string;
		Icon: Component;
		meta?: string;
		width?: SettingsPageWidth;
		actions?: Snippet;
		children: Snippet;
	} = $props();
</script>

<div class="flex min-h-0 flex-1 flex-col">
	<header class="flex flex-none flex-col gap-3 border-b border-line-default px-4 py-3 sm:flex-row sm:items-center sm:px-5">
		<div class="flex min-w-0 flex-1 items-start gap-3">
			<div class="flex size-8 shrink-0 items-center justify-center rounded-md border border-line-default bg-paper-1 text-muted-foreground">
				<Icon class="size-4" aria-hidden="true" />
			</div>
			<div class="flex min-w-0 flex-col gap-0.5">
				<div class="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
					<h1 class="text-md font-medium tracking-snug text-ink-900">{title}</h1>
					{#if meta}
						<span class="font-mono text-2xs text-muted-foreground">{meta}</span>
					{/if}
				</div>
				<p class="text-xs leading-normal text-muted-foreground text-pretty">{description}</p>
			</div>
		</div>

		{#if actions}
			<div class="flex min-w-0 flex-wrap items-center gap-2 sm:justify-end">
				{@render actions()}
			</div>
		{/if}
	</header>

	<div class="flex-1 overflow-auto">
		<main class={settingsPageContent({ width })}>
			{@render children()}
		</main>
	</div>
</div>

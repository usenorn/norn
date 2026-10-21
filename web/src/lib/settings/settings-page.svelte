<script lang="ts">
	import type { Component, Snippet } from "svelte";
	import SettingsBody, { settingsWidth, type SettingsWidth } from "./settings-body.svelte";

	let {
		title,
		description,
		Icon,
		meta,
		width = "standard",
		actions,
		toolbar,
		children,
	}: {
		title: string;
		description: string;
		Icon: Component;
		meta?: string;
		width?: SettingsWidth;
		actions?: Snippet;
		toolbar?: Snippet;
		children: Snippet;
	} = $props();
</script>

<div class="flex min-h-0 flex-1 flex-col">
	<header class="flex-none border-b border-line-default">
		<div class="mx-auto w-full max-w-272 px-4 py-3 sm:px-6 lg:px-8">
			<div
				class={settingsWidth({ width, class: "flex flex-col gap-3 sm:flex-row sm:items-center" })}
			>
				<div class="flex min-w-0 flex-1 items-start gap-3">
					<div
						class="flex size-8 shrink-0 items-center justify-center rounded-md border border-line-default bg-paper-1 text-muted-foreground"
					>
						<Icon class="size-4" aria-hidden="true" />
					</div>
					<div class="flex min-w-0 flex-col gap-0.5">
						<div class="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
							<h1 class="text-md font-semibold tracking-snug text-ink-900">{title}</h1>
							{#if meta}
								<span class="tabular-nums text-2xs text-muted-foreground">{meta}</span>
							{/if}
						</div>
						<p class="text-sm leading-normal text-muted-foreground text-pretty">{description}</p>
					</div>
				</div>

				{#if actions}
					<div class="flex min-w-0 flex-wrap items-center gap-2 sm:justify-end">
						{@render actions()}
					</div>
				{/if}
			</div>
		</div>
	</header>

	{#if toolbar}
		{@render toolbar()}
	{/if}

	<SettingsBody {width}>
		{@render children()}
	</SettingsBody>
</div>

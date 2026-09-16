<script lang="ts">
	import { Command as CommandPrimitive, useId } from "bits-ui";
	import { cn } from "$lib/utils.js";

	let {
		ref = $bindable(null),
		class: className,
		children,
		heading,
		meta,
		value,
		...restProps
	}: CommandPrimitive.GroupProps & {
		heading?: string;
		meta?: string;
	} = $props();
</script>

<CommandPrimitive.Group
	bind:ref
	data-slot="command-group"
	class={cn("overflow-hidden text-foreground", className)}
	value={value ?? heading ?? `----${useId()}`}
	{...restProps}
>
	{#if heading}
		<CommandPrimitive.GroupHeading
			class="flex items-center gap-1.5 px-2 pt-1.5 pb-1 font-mono text-2xs font-medium tracking-eyebrow text-ink-600 uppercase"
		>
			{heading}
			{#if meta}
				<span class="ml-auto text-2xs tracking-normal text-muted-foreground normal-case">{meta}</span>
			{/if}
		</CommandPrimitive.GroupHeading>
	{/if}
	<CommandPrimitive.GroupItems {children} />
</CommandPrimitive.Group>

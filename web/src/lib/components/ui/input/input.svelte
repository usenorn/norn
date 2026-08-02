<script lang="ts" module>
	export const inputClass =
		"h-control-md w-full min-w-0 rounded-none border-0 border-b border-input bg-transparent px-1 text-md text-ink-900 outline-none transition-colors duration-110 ease-out placeholder:text-muted-foreground hover:border-ink-400 hover:bg-accent focus-visible:border-ring focus-visible:bg-accent focus-visible:rule-under aria-invalid:border-destructive aria-invalid:[--rule-under-color:var(--destructive)] disabled:pointer-events-none disabled:border-dashed disabled:text-ink-300 file:inline-flex file:h-control-sm file:border-0 file:bg-transparent file:text-sm file:font-medium file:text-foreground";
</script>

<script lang="ts">
	import { cn, type WithElementRef } from "$lib/utils.js";
	import type { HTMLInputAttributes, HTMLInputTypeAttribute } from "svelte/elements";

	type InputType = Exclude<HTMLInputTypeAttribute, "file">;

	type Props = WithElementRef<
		Omit<HTMLInputAttributes, "type"> &
			({ type: "file"; files?: FileList } | { type?: InputType; files?: undefined })
	>;

	let {
		ref = $bindable(null),
		value = $bindable(),
		type,
		files = $bindable(),
		class: className,
		"data-slot": dataSlot = "input",
		...restProps
	}: Props = $props();
</script>

{#if type === "file"}
	<input
		bind:this={ref}
		data-slot={dataSlot}
		class={cn(inputClass, className)}
		type="file"
		bind:files
		bind:value
		{...restProps}
	/>
{:else}
	<input
		bind:this={ref}
		data-slot={dataSlot}
		class={cn(inputClass, className)}
		{type}
		bind:value
		{...restProps}
	/>
{/if}

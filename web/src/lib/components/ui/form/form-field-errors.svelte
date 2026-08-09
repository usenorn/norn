<script lang="ts">
	import * as FormPrimitive from "formsnap";
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import { cn, type WithoutChild } from "$lib/utils.js";

	let {
		ref = $bindable(null),
		class: className,
		errorClasses,
		children: childrenProp,
		...restProps
	}: WithoutChild<FormPrimitive.FieldErrorsProps> & {
		errorClasses?: string | undefined | null;
	} = $props();
</script>

<FormPrimitive.FieldErrors
	bind:ref
	class={cn("animate-mark flex flex-col gap-1 text-sm text-destructive", className)}
	{...restProps}
>
	{#snippet children({ errors, errorProps })}
		{#if childrenProp}
			{@render childrenProp({ errors, errorProps })}
		{:else}
			{#each errors as error (error)}
				<div {...errorProps} class={cn("flex items-center gap-1.5", errorClasses)}>
					<CircleAlert class="size-icon-row shrink-0" aria-hidden="true" />
					{error}
				</div>
			{/each}
		{/if}
	{/snippet}
</FormPrimitive.FieldErrors>

<script lang="ts">
	let {
		colors,
		labels,
		value,
		name,
		disabled = false,
		onpick,
		...rest
	}: {
		colors: readonly string[];
		labels: Record<string, string>;
		value: string;
		name: string;
		disabled?: boolean;
		onpick: (color: string) => void;
		[key: string]: unknown;
	} = $props();
</script>

<div {...rest} class="flex flex-wrap gap-2">
	{#each colors as color (color)}
		<button
			type="button"
			{disabled}
			aria-pressed={value === color}
			aria-label={labels[color]}
			onclick={() => onpick(color)}
			class="inline-flex items-center gap-1.5 rounded-sm border px-2 py-1 text-sm motion-control disabled:opacity-50 aria-pressed:border-primary aria-pressed:bg-accent"
			class:border-line-default={value !== color}
		>
			<span
				class="size-2.5 rounded-xs"
				style="background: var(--label-{color})"
				aria-hidden="true"
			></span>
			{labels[color]}
		</button>
	{/each}
</div>
<input type="hidden" {name} {value} />

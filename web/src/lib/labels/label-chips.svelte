<script lang="ts" module>
	import { tv, type VariantProps } from "tailwind-variants";

	export const labelChipsVariants = tv({
		slots: {
			root: "flex min-w-0 items-center gap-1.5",
			tag: "",
			more: "font-mono text-2xs whitespace-nowrap text-muted-foreground",
		},
		variants: {
			place: {
				row: { root: "contents", tag: "hidden lg:inline-flex", more: "hidden lg:inline" },
				card: {},
				field: { root: "flex-1 flex-wrap" },
			},
		},
		defaultVariants: { place: "card" },
	});

	export type LabelChipsPlace = NonNullable<VariantProps<typeof labelChipsVariants>["place"]>;
</script>

<script lang="ts">
	import Tag from "$lib/components/norn/tag.svelte";
	import type { LabelColor } from "./labels";

	let {
		labels,
		place = "card",
	}: {
		labels: { id?: string; name: string; color: LabelColor }[];
		place?: LabelChipsPlace;
	} = $props();

	const shownAtMost = 2;

	const styles = $derived(labelChipsVariants({ place }));
	const visible = $derived(place === "field" ? labels : labels.slice(0, shownAtMost));
	const hidden = $derived(labels.length - visible.length);
</script>

<span class={styles.root()}>
	{#each visible as label (label.id ?? label.name)}
		<Tag name={label.name} color={label.color} class={styles.tag()} />
	{/each}
	{#if hidden > 0}
		<span class={styles.more()}>+{hidden}</span>
	{/if}
</span>

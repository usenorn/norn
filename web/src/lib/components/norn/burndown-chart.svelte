<script lang="ts">
	import { Grid, LineChart, Points, Spline } from "layerchart";
	import * as Chart from "$lib/components/ui/chart";
	import type { BurndownDay, BurndownSeries } from "$lib/cycles/cycles";
	import { onCalendarDate } from "$lib/time";

	let { series, label }: { series: BurndownSeries; label: string } = $props();

	const config = {
		remaining: { label: "Remaining", color: "var(--color-primary)" },
		ideal: { label: "Ideal", color: "var(--color-line-strong)" },
	} satisfies Chart.ChartConfig;

	const gridlines = $derived([0.25, 0.5, 0.75].map((share) => series.ceiling * share));
	const xDomain = $derived<[number, number]>(series.span > 1 ? [0, series.span - 1] : [-1, 1]);

	function headline(at: number): string {
		const day: BurndownDay | undefined = series.days[at];

		if (!day) return "";
		if (day.scope === null) return onCalendarDate(day.on);

		return `${onCalendarDate(day.on)} · scope ${day.scope}`;
	}
</script>

{#snippet amount({ value, name, item }: { value: unknown; name: string; item: { key: string } })}
	<span class="flex flex-1 items-center justify-between gap-4 leading-none">
		<span class="text-muted-foreground">{name}</span>
		<span class="font-mono font-medium text-foreground tabular-nums">
			{#if value === null || value === undefined}
				—
			{:else if item.key === "ideal"}
				{Math.round(Number(value))}
			{:else}
				{Number(value)}
			{/if}
		</span>
	</span>
{/snippet}

<Chart.Container {config} class="aspect-auto h-32 w-full" role="img" aria-label={label}>
	<LineChart
		data={series.days}
		x="at"
		{xDomain}
		yDomain={[0, series.ceiling]}
		padding={{ top: 4, bottom: 1 }}
		axis={false}
		grid={false}
		rule={false}
		series={[
			{ key: "remaining", label: config.remaining.label, color: config.remaining.color },
			{ key: "ideal", label: config.ideal.label, color: config.ideal.color },
		]}
	>
		{#snippet marks()}
			<Grid y yTicks={gridlines} classes={{ line: "stroke-line-default!" }} />
			<Grid y yTicks={[0]} classes={{ line: "stroke-line-strong!" }} />
			{#if series.hasIdeal}
				<Spline seriesKey="ideal" class="stroke-line-strong" />
			{/if}
			<Spline
				seriesKey="remaining"
				defined={(day: BurndownDay) => day.remaining !== null}
				class="stroke-primary stroke-2"
			/>
			<Points data={series.isolated} y="remaining" r={3} class="fill-primary stroke-transparent" />
		{/snippet}

		{#snippet tooltip()}
			<Chart.Tooltip
				indicator="line"
				labelFormatter={(at) => headline(Number(at))}
				formatter={amount}
			/>
		{/snippet}
	</LineChart>
</Chart.Container>

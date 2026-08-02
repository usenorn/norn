<script lang="ts">
	import ArrowRight from "@lucide/svelte/icons/arrow-right";
	import Check from "@lucide/svelte/icons/check";
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import * as Avatar from "$lib/components/ui/avatar/index.js";
	import * as Select from "$lib/components/ui/select/index.js";
	import * as Table from "$lib/components/ui/table/index.js";
	import type { MappingRow, PersonRow } from "$lib/import/types";

	export type MappingSection = { name: string | null; rows: (MappingRow | PersonRow)[] };

	let {
		columnLabel,
		sections,
		identity = "field",
	}: {
		columnLabel: string;
		sections: MappingSection[];
		identity?: "field" | "person";
	} = $props();

	let chosen = $state<Record<string, string>>({});

	function initials(name: string) {
		return name
			.split(/\s+/)
			.slice(0, 2)
			.map((part) => part[0] ?? "")
			.join("");
	}
</script>

<Table.Root class="min-w-[560px]">
	<Table.Header>
		<Table.Row>
			<Table.Head>{columnLabel}</Table.Head>
			<Table.Head data-align="right">Volume</Table.Head>
			<Table.Head class="w-11"><span class="sr-only">Becomes</span></Table.Head>
			<Table.Head class="w-[230px]">In Norn</Table.Head>
		</Table.Row>
	</Table.Header>
	<Table.Body>
		{#each sections as section (section.name ?? "rows")}
			{#if section.name}
				<Table.Row>
					<Table.Cell colspan={4} class="h-6.5 bg-paper-2">
						<span class="font-mono text-2xs tracking-eyebrow text-ink-600 uppercase">
							{section.name}
						</span>
					</Table.Cell>
				</Table.Row>
			{/if}
			{#each section.rows as row (row.source)}
				{@const pending = row.needsDecision && chosen[row.source] === undefined}
				{@const value = chosen[row.source] ?? row.value}
				<Table.Row>
					<Table.Cell class="max-w-0">
						<span class="flex min-w-0 items-center gap-2">
							{#if identity === "person"}
								<Avatar.Root size="xs" variant={pending ? "ghost" : "default"}>
									<Avatar.Fallback>{pending ? "" : initials(row.source)}</Avatar.Fallback>
								</Avatar.Root>
							{:else if pending}
								<CircleAlert class="size-icon-row shrink-0 text-warning" aria-hidden="true" />
							{:else}
								<Check class="size-icon-row shrink-0 text-muted-foreground" aria-hidden="true" />
							{/if}
							<span
								class="truncate text-sm text-ink-900 {identity === 'person'
									? 'font-sans'
									: 'font-mono'}"
							>
								{row.source}
							</span>
							{#if "email" in row}
								<span class="truncate font-mono text-xs text-muted-foreground">{row.email}</span>
							{/if}
						</span>
					</Table.Cell>
					<Table.Cell
						data-align="right"
						class="font-mono text-sm text-muted-foreground tabular-nums"
					>
						{row.volume}
					</Table.Cell>
					<Table.Cell>
						<ArrowRight
							class="size-icon-row {pending ? 'text-warning' : 'text-ink-300'}"
							aria-hidden="true"
						/>
					</Table.Cell>
					<Table.Cell>
						<Select.Root
							type="single"
							value={chosen[row.source] ?? (row.needsDecision ? "" : row.value)}
							onValueChange={(next) => (chosen[row.source] = next)}
						>
							<Select.Trigger
								size="sm"
								aria-label="{row.source} becomes"
								aria-invalid={pending ? "true" : undefined}
							>
								{value}
							</Select.Trigger>
							<Select.Content>
								{#each row.options as option (option)}
									<Select.Item value={option} label={option} />
								{/each}
							</Select.Content>
						</Select.Root>
					</Table.Cell>
				</Table.Row>
			{/each}
		{/each}
	</Table.Body>
</Table.Root>

<script lang="ts">
	import Bot from "@lucide/svelte/icons/bot";
	import CircleDot from "@lucide/svelte/icons/circle-dot";
	import FolderKanban from "@lucide/svelte/icons/folder-kanban";
	import User from "@lucide/svelte/icons/user";
	import Users from "@lucide/svelte/icons/users";

	export type PopupRow = {
		key: string;
		label: string;
		hint: string;
		group: string;
		icon?: "person" | "agent" | "team" | "issue" | "project";
		shortcut?: string;
	};

	let {
		id,
		rows,
		index,
		left,
		top,
		state = "ready",
		label = "Suggestions",
		typingHint = "Keep typing to search",
		optionId,
		onpick,
	}: {
		id: string;
		rows: PopupRow[];
		index: number;
		left: number;
		top: number;
		state?: "ready" | "loading" | "typing" | "empty" | "failed";
		label?: string;
		typingHint?: string;
		optionId: (at: number) => string;
		onpick: (at: number) => void;
	} = $props();

	const icons = {
		person: User,
		agent: Bot,
		team: Users,
		issue: CircleDot,
		project: FolderKanban,
	} as const;

	const grouped = $derived.by(() => {
		const groups: { key: string; label: string; rows: { row: PopupRow; at: number }[] }[] = [];

		rows.forEach((row, at) => {
			const last = groups.at(-1);

			if (last && last.label === row.group) {
				last.rows.push({ row, at });

				return;
			}

			groups.push({ key: `${row.group}:${row.key}`, label: row.group, rows: [{ row, at }] });
		});

		return groups;
	});
</script>

<div
	class="fixed z-50 max-h-64 w-80 max-w-[calc(100vw-2rem)] overflow-y-auto overscroll-contain rounded-md border border-line-strong bg-popover p-1 shadow-md"
	style="left: {left}px; top: {top}px"
>
	{#if state === "loading"}
		<p class="px-2 py-1.5 text-sm text-muted-foreground">Searching…</p>
	{:else if state === "typing"}
		<p class="px-2 py-1.5 text-sm text-muted-foreground">{typingHint}</p>
	{:else if state === "failed"}
		<p class="px-2 py-1.5 text-sm text-muted-foreground">
			Search is unavailable just now. Keep typing and try again.
		</p>
	{:else if rows.length === 0}
		<p class="px-2 py-1.5 text-sm text-muted-foreground">No matches</p>
	{/if}

	<ul {id} role="listbox" aria-label={label} class="min-w-0">
		{#each grouped as group (group.key)}
			{#if group.label !== ""}
				<li
					role="presentation"
					class="px-2 pt-1.5 pb-0.5 text-2xs font-medium tracking-wide text-muted-foreground uppercase"
				>
					{group.label}
				</li>
			{/if}
			{#each group.rows as held (held.row.key)}
				{@const Icon = held.row.icon ? icons[held.row.icon] : null}
				<li>
					<button
						type="button"
						id={optionId(held.at)}
						role="option"
						aria-selected={held.at === index}
						tabindex="-1"
						class="flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-left text-sm {held.at ===
						index
							? 'bg-accent text-ink-900'
							: 'text-ink-900'}"
						onmousedown={(event) => {
							event.preventDefault();
							onpick(held.at);
						}}
					>
						{#if Icon}
							<Icon class="size-3.5 shrink-0 text-muted-foreground" aria-hidden="true" />
						{/if}
						<span class="min-w-0 flex-1 truncate">{held.row.label}</span>
						{#if held.row.shortcut}
							<kbd class="shrink-0 font-mono text-2xs text-muted-foreground">{held.row.shortcut}</kbd>
						{:else if held.row.hint}
							<span class="max-w-32 shrink-0 truncate font-mono text-2xs text-muted-foreground">
								{held.row.hint}
							</span>
						{/if}
					</button>
				</li>
			{/each}
		{/each}
	</ul>
</div>

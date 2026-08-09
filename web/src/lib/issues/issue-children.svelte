<script lang="ts">
	import * as Avatar from "$lib/components/ui/avatar/index.js";
	import StatusIcon from "$lib/components/norn/status-icon.svelte";
	import PropertyPicker from "$lib/issues/property-picker.svelte";
	import { initialsOf } from "$lib/team/members";
	import { onDayMonth } from "$lib/time";
	import type { WorkflowState } from "$lib/team/states";
	import type { Issue } from "$lib/issues/issues";

	let {
		children,
		at,
		states = [],
		now,
		timezone,
		nameOf,
		working = false,
		onstate,
	}: {
		children: Issue[];
		at: (path: string) => string;
		states?: WorkflowState[];
		now: string;
		timezone: string;
		nameOf: (accountId: string | undefined) => string;
		working?: boolean;
		onstate?: (child: Issue, stateId: string) => void;
	} = $props();

	const movable = $derived(Boolean(onstate) && states.length > 0);
</script>

{#if children.length === 0}
	<p class="text-md text-muted-foreground">Nothing is filed under this issue yet.</p>
{:else}
	<ul class="flex flex-col">
		{#each children as child (child.id)}
			<li class="group/child relative flex h-row items-center gap-2.25 border-b border-line-subtle">
				{#if movable && onstate}
					<span class="relative z-1 flex flex-none">
						<PropertyPicker
							options={states.map((state) => ({
								value: state.id,
								label: state.name,
								checked: state.id === child.state.id,
							}))}
							placeholder="Set status…"
							onpick={(stateId) => onstate(child, stateId)}
						>
							{#snippet trigger(props)}
								<button
									{...props}
									type="button"
									disabled={working}
									aria-label="Change status on {child.reference}"
									class="inline-flex h-6 w-5.5 cursor-pointer items-center justify-center rounded-sm motion-control hover:bg-paper-2 aria-expanded:bg-paper-2"
								>
									<StatusIcon category={child.state.category} name={child.state.name} />
								</button>
							{/snippet}
							{#snippet mark(option)}
								{@const state = states.find((candidate) => candidate.id === option.value)}
								{#if state}
									<StatusIcon category={state.category} decorative />
								{/if}
							{/snippet}
						</PropertyPicker>
					</span>
				{:else}
					<StatusIcon category={child.state.category} name={child.state.name} />
				{/if}

				<a
					href={at(`/issues/${child.reference}`)}
					class="flex min-w-0 flex-1 items-center gap-2.25 after:absolute after:inset-0 after:-mx-1 after:rounded-sm after:motion-control group-hover/child:after:bg-accent"
				>
					<span class="relative z-1 font-mono text-xs text-muted-foreground">
						{child.reference}
					</span>
					<span
						class="relative z-1 min-w-0 flex-1 truncate text-md {child.state.category === 'complete'
							? 'text-muted-foreground line-through decoration-line-strong'
							: 'text-ink-900'}"
					>
						{child.title}
					</span>
					{#if child.dueOn}
						<span class="relative z-1 font-mono text-2xs whitespace-nowrap text-muted-foreground">
							{onDayMonth(child.dueOn, now, timezone)}
						</span>
					{/if}
					{#if nameOf(child.assigneeAccountId)}
						<Avatar.Root size="xs" class="relative z-1">
							<Avatar.Fallback>{initialsOf(nameOf(child.assigneeAccountId))}</Avatar.Fallback>
						</Avatar.Root>
					{:else}
						<span
							class="relative z-1 size-icon-row rounded-full border border-dashed border-line-strong"
							aria-label="Unassigned"
						></span>
					{/if}
				</a>
			</li>
		{/each}
	</ul>
{/if}
